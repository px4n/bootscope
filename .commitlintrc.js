module.exports = {
  extends: ["@commitlint/config-conventional"],
  rules: {
    "type-enum": [2, "always", ["feat", "fix", "docs", "style", "refactor", "perf", "test", "chore", "ci", "revert"]],
    "subject-case": [2, "never", ["upper-case", "sentence-case", "start-case"]],
    "subject-full-stop": [2, "never", "."],
    "subject-empty": [2, "never"],
    "type-empty": [2, "never"],
    "scope-case": [2, "always", "lower-case"],
    "body-leading-blank": [1, "always"],
    "footer-leading-blank": [1, "always"],
  },
  ignores: [(commit) => /^build\(deps\): bump.*/i.test(commit)],
};
