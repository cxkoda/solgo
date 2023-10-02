// Package ipfs abstracts the IPFS implementation, Kubo, exposing commonly
// required functionality without the need for in-depth understanding of the
// IPFS API.
package ipfs

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"sync"
	"sync/atomic"

	"github.com/ipfs/go-libipfs/files"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/plugin/loader"
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// LoadPlugins loads and initialises all plugins from the repoPath, which MAY be
// an empty string, in which case only "preload" plugins are loaded. It MUST be
// called before either InitFS() or FromFS(). If calling before InitFS(), the
// repoPath SHOULD be the empty string; if calling before FromFS(), the repoPath
// passed to each function MUST be the same.
func LoadPlugins(repoPath string) (_ *loader.PluginLoader, retErr error) {
	l, err := loader.NewPluginLoader(repoPath)
	if err != nil {
		return nil, fmt.Errorf("loader.NewPluginLoader(%q): %v", repoPath, err)
	}
	defer func() {
		if retErr != nil {
			l.Close()
		}
	}()

	if err := l.Initialize(); err != nil {
		return nil, fmt.Errorf("%T.Initialize(): %v", l, err)
	}
	if err := l.Inject(); err != nil {
		return nil, fmt.Errorf("%T.Inject(): %v", l, err)
	}
	return l, nil
}

// InitFS initialises a filesystem-based repository at the specified path.
//
// Note that the filesystem refers to the IPFS repository layout, not the
// content to be served.
func InitFS(repoPath string, cfg *config.Config) error {
	if err := fsrepo.Init(repoPath, cfg); err != nil {
		return fmt.Errorf("fsrepo.Init(%q, %T): %v", repoPath, cfg, err)
	}
	return nil
}

// IPFS wraps an implementation of the core IPFS API, providing convenience
// methods for common functionality.
type IPFS struct {
	iface.CoreAPI
	node *core.IpfsNode
}

// FromFS returns an IPFS instance using a filesystem-based node at the specified
// path. The same path MUST have already been initialised with InitFS() or
// similar functionality, such as the CLI command `ipfs init`.
//
// See the note on InitFS() re filesystem.
func FromFS(ctx context.Context, repoPath string, nodeCfg *core.BuildCfg, opts ...options.ApiOption) (*IPFS, error) {
	if nodeCfg.NilRepo {
		return nil, fmt.Errorf("%T.NilRepo MUST be false when creating from FS", nodeCfg)
	}
	if nodeCfg.Repo != nil {
		return nil, fmt.Errorf("%T.Repo MUST be nil (to be overwritten) when creating from FS; got %T", nodeCfg, nodeCfg.Repo)
	}

	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, fmt.Errorf("fsrepo.Open(%q): %v", repoPath, err)
	}
	nodeCfg.Repo = repo

	node, err := core.NewNode(ctx, nodeCfg)
	if err != nil {
		return nil, fmt.Errorf("core.NewNode(…, %+v): %v", nodeCfg, err)
	}

	api, err := coreapi.NewCoreAPI(node, opts...)
	if err != nil {
		return nil, fmt.Errorf("coreapi.NewCoreAPI(%T, …): %v", node, err)
	}
	return &IPFS{
		CoreAPI: api,
		node:    node,
	}, nil
}

// DefaultPeers are a commonly used set of default peers to use for
// bootstrapping p2p connections. They are a parsed version of the default
// addresses provided by Kubo's config package.
var DefaultPeers []*peer.AddrInfo

func init() {
	if err := ResetDefaultPeers(); err != nil {
		// This can only happen if the default values in ResetDefaultPeers()
		// are incorrectly formatted.
		panic(err)
	}
}

// ResetDefaultPeers resets the global DefaultPeers variable, adding optional
// extra addresses. See ParsePeerAddresses() for treatment of the extra
// addresses.
//
// Note: see the security advice re adding extra nodes:
// https://docs.ipfs.tech/how-to/modify-bootstrap-list/.
func ResetDefaultPeers(extra ...string) error {
	peers, err := ParsePeerAddresses(append(config.DefaultBootstrapAddresses, extra...)...)
	if err != nil {
		return err
	}
	DefaultPeers = peers
	return nil
}

// PrasePeerAdddresses parses the address strings as multiaddr.NewMultiAddr
// values, groups them by peer.ID, and returns the groups as libp2p
// peer.AddrInfos. The adress strings MAY therefore have overlapping peer IDs.
func ParsePeerAddresses(addrs ...string) ([]*peer.AddrInfo, error) {
	peerAddrs := make(map[peer.ID][]multiaddr.Multiaddr)

	for _, addr := range addrs {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("multiaddr.NewMultiaddr(%q): %v", addr, err)
		}

		ai, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			return nil, fmt.Errorf("peer.AddrInfoFromP2pAddr(): %v", err)
		}
		peerAddrs[ai.ID] = append(peerAddrs[ai.ID], ai.Addrs...)
	}

	peers := make([]*peer.AddrInfo, 0, len(peerAddrs))
	for id, addrs := range peerAddrs {
		peers = append(peers, &peer.AddrInfo{
			ID:    id,
			Addrs: addrs,
		})
	}
	return peers, nil
}

// TryConnect attempts to connect to all provided peers, typically expecting
// DefaultPeers, and returns the number of successful connections. If at least
// one connection is successful, the p2p network will begin sharing other peer
// information. See ipfs.Swarm().Peers() for the number of connections, even
// before TryConnect() returns.
func (ipfs *IPFS) TryConnect(ctx context.Context, peers []*peer.AddrInfo) uint64 {
	var (
		success uint64
		wg      sync.WaitGroup
	)

	wg.Add(len(peers))
	for _, p := range peers {
		go func(p *peer.AddrInfo) {
			defer wg.Done()
			if ipfs.Swarm().Connect(ctx, *p) == nil {
				atomic.AddUint64(&success, 1)
			}
		}(p)
	}
	wg.Wait()

	return success
}

// WaitForConnections repeatedly checks for all connected peers and returns when
// either the Context is Done() or the number of peers exceeds the minimum.
func (ipfs *IPFS) WaitForConnections(ctx context.Context, min int) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			p, err := ipfs.Swarm().Peers(ctx)
			if err == nil && len(p) >= min {
				return nil
			}
		}
	}
}

// FSRootHandling defines how to treat the root of an fs.FS when adding its
// contents.
type FSRootHandling bool

const (
	// KeepFSRoot results in files being added with the root directory
	// maintained, such that all files have it as a path prefix.
	KeepFSRoot FSRootHandling = true
	// StripFSRoot is the opposite of KeepFSRoot, equivalent to adding an fs.FS
	// obtained via `fs.Sub(…,root)`, thus stripping the root prefix.
	StripFSRoot FSRootHandling = false
)

// AddFS walks the provided filesystem from the provided root, collecting all
// non-directory entries, and adding them to ipfs.Unixfs() as a files.Directory.
//
// Unlike InitFS() and NewFS() that refer to a filesystem-based IPFS repository,
// this method refers to the actual files being added to the system.
func (ipfs *IPFS) AddFS(ctx context.Context, fsys fs.FS, root string, rh FSRootHandling, opts ...options.UnixfsAddOption) (path.Resolved, error) {
	var close []io.Closer
	defer func() {
		for _, c := range close {
			// It's safe to ignore errors returned here because we're only
			// reading from files.
			c.Close()
		}
	}()

	if rh == StripFSRoot {
		sub, err := fs.Sub(fsys, root)
		if err != nil {
			return nil, fmt.Errorf("stripping FS root: fs.Sub(%T, %q): %v", fsys, root, err)
		}
		fsys = sub
		root = "."
	}

	ipfsDir := make(map[string]files.Node)
	fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("fs.WalkDirFunc received %w", err)
		}
		if d.IsDir() {
			return nil
		}

		f, err := fsys.Open(path)
		if err != nil {
			return fmt.Errorf("%T.Open(%q): %v", fsys, path, err)
		}
		close = append(close, f)
		ipfsDir[path] = files.NewReaderFile(f)
		return nil
	})

	return ipfs.Unixfs().Add(ctx, files.NewMapDirectory(ipfsDir), opts...)
}
