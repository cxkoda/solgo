# Automatic Go documentation

Start a `godoc` server at http://localhost:6060/pkg/github.com/cxkoda/,
including generated files such as `*.pb.go` and Solidity contract bindings:

```Shell
bazel build //devtools/godoc:goroot && \
bazel run //devtools/godoc -- \
    --goroot=$(bazel info bazel-bin)/devtools/godoc/goroot \
    --index \
    --http=:6060
```

Alternatively, run `devtools/godoc.sh`.