package ipfs

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-libipfs/files"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
)

//go:embed testdata/*
var testdata embed.FS

// fromCleanTmpDir initialises and returns a new repo in a fresh temporary
// directory.
func fromCleanTmpDir(ctx context.Context, t *testing.T) *IPFS {
	t.Helper()

	repo, err := os.MkdirTemp("", "ipfs-roundtrip-test-*")
	if err != nil {
		t.Fatalf("os.MkdirTemp(…) error %v", err)
	}

	cfg, err := config.Init(io.Discard, 2048)
	if err != nil {
		t.Fatalf("config.Init(…) error %v", err)
	}
	if err := InitFS(repo, cfg); err != nil {
		t.Fatalf("InitFS(%q, nil) error %v", repo, err)
	}

	nodeCfg := &core.BuildCfg{
		Online:                      true,
		DisableEncryptedConnections: true, // only safe for testing
	}
	ipfs, err := FromFS(ctx, repo, nodeCfg)
	if err != nil {
		t.Fatalf("FromFS(ctx, %q, %+v) error %v", repo, err, nodeCfg)
	}
	t.Logf("Node %v backed by %q", ipfs.node.Identity, repo)
	return ipfs
}

func TestLocalhostRoundtrip(t *testing.T) {
	ctx := context.Background()

	plugins, err := LoadPlugins("")
	if err != nil {
		t.Fatalf("InitPlugins(%q) error %v", "", err)
	}
	t.Cleanup(func() {
		if err := plugins.Close(); err != nil {
			t.Errorf("%T.Close() error %v", plugins, err)
		}
	})

	// Spawning nodes with networking enabled takes ~10s so do it concurrently.
	var (
		primary, secondary *IPFS
		wg                 sync.WaitGroup
	)
	for _, node := range []**IPFS{&primary, &secondary} {
		wg.Add(1)
		go func(n **IPFS) {
			defer wg.Done()
			*n = fromCleanTmpDir(ctx, t)
		}(node)
	}
	wg.Wait()

	ipfs := primary
	fsCID, err := ipfs.AddFS(ctx, testdata, "testdata", StripFSRoot)
	if err != nil {
		t.Fatalf("%T.AddFS(ctx, %T{testdata/*}, %q) error %v", ipfs, testdata, "testdata", err)
	}

	t.Run("fs.FS CID", func(t *testing.T) {
		got := fsCID.Cid()

		// From running `ipfs add -r .` in the `testdata` directory.
		const wantStr = "QmX9MfavkGfNUYAhGouW4wXoYPu3uuszG11NCD1mAC6VVB"
		want, err := cid.Decode(wantStr)
		if err != nil {
			t.Fatalf("Bad test setup; cid.Decode(%q) error %v", wantStr, err)
		}

		if !got.Equals(want) {
			t.Errorf("%T.AddFS(ctx, %T{testdata/*}, %q) got CID %v; want %v", ipfs, testdata, "testdata", got, want)
		}
	})

	t.Run("retrieve files through secondary node", func(t *testing.T) {
		// Generally one would use DefaultPeers to connect to the actual libp2p
		// network, but for testing we only want to connect to the fresh node
		// created above.
		addr := fmt.Sprintf("/ip4/127.0.0.1/tcp/4242/p2p/%s", ipfs.node.Identity.String())
		peers, err := ParsePeerAddresses(addr)
		if err != nil {
			t.Fatalf("ParsePeerAddresses(%q) error %v", addr, err)
		}
		// Guarantee that we have no direct access to the original node and must
		// therefore be going via the secondary.
		primary = nil
		ipfs = secondary

		if got, want := ipfs.TryConnect(ctx, peers), uint64(1); got != want {
			t.Fatalf("%T.TryConnect(ctx, %+v) got %d; want %d", ipfs, peers, got, want)
		}
		if err := ipfs.WaitForConnections(ctx, 1); err != nil {
			t.Fatalf("%T.WaitForConnections(ctx, 1) error %v", ipfs, err)
		}

		got := make(map[string][]byte)
		want := map[string][]byte{
			"foo.txt": []byte("foo"),
			"bar.txt": []byte("bar"),
			"leet":    {1, 3, 3, 7},
		}

		entries, err := ipfs.Unixfs().Ls(ctx, fsCID)
		if err != nil {
			t.Fatalf("%T.Unixfs().Ls(ctx, %v) error %v", ipfs, fsCID, err)
		}
		for e := range entries {
			t.Run(e.Name, func(t *testing.T) {
				if e.Err != nil {
					t.Fatalf("%T.Err = %v", e, e.Err)
				}
				if e.Type != iface.TFile {
					t.Fatalf("got %T.Type = %v; want %v", e, e.Type, iface.TFile)
				}

				node, err := ipfs.Unixfs().Get(ctx, path.IpfsPath(e.Cid))
				if err != nil {
					t.Fatalf("%T.Unixfs().Get(ctx, path.IpfsPath(%v)) error %v", ipfs, e.Cid, err)
				}
				f, ok := node.(files.File)
				if !ok {
					t.Fatalf("%T.Unixfs().Get(ctx, %v) got %T; want files.File", ipfs, e.Cid, node)
				}

				buf, err := io.ReadAll(f)
				if err != nil {
					t.Fatalf("io.ReadAll(%T.Unixfs().Get(ctx, %v)) error %v", ipfs, e.Cid, err)
				}
				got[e.Name] = buf
			})
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Get() all files from %T.Unixfs().Ls(ctx, %v) diff (-want +got):\n%s", ipfs, fsCID, diff)
		}
	})
}
