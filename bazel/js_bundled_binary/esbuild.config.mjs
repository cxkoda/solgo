const resolver = (build) => {
  // Resolves all paths beginning with the @proof prefix relative to the
  // bazel-out/<platform>/bin/ directory.
  const prefix = "@proof/";
  const filter = new RegExp(prefix + ".*");
  const binDirPattern = /(.*\/bazel-out\/.+?\/bin\/)/;

  build.onResolve({ filter }, async (args) => {
    const binDir = args.resolveDir.match(binDirPattern);
    return {
      path: binDir[0] + args.path.substring(prefix.length) + ".js",
    };
  });
};

export default {
  plugins: [
    {
      name: "proof-workspace-resolver",
      setup: resolver,
    },
  ],
};
