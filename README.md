# Autodeployer

## Side note

- By default, the script assumes you don't want to make a new release after someone else has made the previous rc
  - Example: Say someone else makes `rc-2` for a certain branch, you now want to make `rc-3` for the same branch. You will need to modify the default behavior of filtering the tags to only include the ones that you have made
- It also assumes that you are making a minor bump to the version
  - Example: Currently latest version of some branch is `1.2.3`, default behavior will be to make `1.2.4-rc*` if you want to make `1.3.0-rc*` or `2.0.0-rc*`, you will need to modify the default behavior
- It assumes that you do not manually create tags for the release candidates without updating the deployment repo. Always make sure that the deployment repo is up to date with the latest rc tag created

## TODO

- [X] Being able to use the same branch for several release candidates
- [ ] Works with other repos
