// The grpc_gen_ts binary reads a FileDescriptorProto on stdin and writes, to
// stdout, protobufjs bindings to use @grpc/grpc-js as a transport.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/golang/glog"
	"google.golang.org/protobuf/proto"
	descpb "google.golang.org/protobuf/types/descriptorpb"

	_ "embed"
)

func main() {
	pbTarget := flag.String("pb_target", "", "Name of protobuf target being generated")
	workspaceRelPath := flag.String("workspace_rel_path", "./", "Relative path to workspace root")
	flag.Parse()

	if err := run(os.Stdin, os.Stdout, *pbTarget, *workspaceRelPath); err != nil {
		glog.Exit(err)
	}
}

var (
	//go:embed servicedef.go.tmpl
	rawTmpl string

	tmplFuncs = template.FuncMap{
		"namespaces": func(pkg string) []string {
			return strings.Split(pkg, ".")
		},
		"join": func(delim string, s ...string) string {
			return strings.Join(s, delim)
		},
		"streaming": func(m *descpb.MethodDescriptorProto) string {
			switch c, s := m.GetClientStreaming(), m.GetServerStreaming(); {
			case c && s:
				return "bidi"
			case c && !s:
				return "client"
			case s && !c:
				return "server"
			default:
				return "unary"
			}
		},
	}

	tmpl = template.Must(
		template.New("grpc_ts").Funcs(tmplFuncs).Parse(rawTmpl),
	)
)

// run reads a raw FileDescritproSet from src and uses it as data to the global
// `tmpl` template, which is Execute()d to dest.
func run(src io.Reader, dest io.Writer, pbTarget, workspaceRelPath string) error {
	files, err := descriptors(src)
	if err != nil {
		return err
	}
	return tmpl.Execute(dest, struct {
		PBTarget         string
		WorkspaceRelPath string
		DescriptorSet    *descpb.FileDescriptorSet
	}{pbTarget, workspaceRelPath, files})
}

// descriptors expects r to contain a marshalled FileDescriptorSet, which it
// reads and unmarshals for return.
func descriptors(r io.Reader) (*descpb.FileDescriptorSet, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("io.ReadAll(): %v", err)
	}

	set := new(descpb.FileDescriptorSet)
	if err := proto.Unmarshal(buf, set); err != nil {
		return nil, fmt.Errorf("proto.Unmarshal(…, %T): %v", set, err)
	}
	return set, nil
}
