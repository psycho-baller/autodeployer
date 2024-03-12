# Autodeployer

## Why use this script?

This script was built to automate the process of creating releases and deploying them to staging. By running a single command, it will take care of creating the release, bumping the deployment repo, and running the deployment workflow, and notify you of the result of the deployment when it is done.

## What happens when you run the script?

When you run the script, it will take ~5 seconds to create the releases and create or bump the deployment repo. It will then run the 'deploy' workflow in the deployment repo. This usually takes around 5-10 minutes. When the workflow is completed, you will be notified by a popup on your screen notifying you of the result of the deployment.

## How to use

There are 4 ways to use this script:

1. Add `raycast.applescript` to your raycast scripts folder and run it from there. Don't forget to set up an alias for the script for easy access. I have it as `ad`, so to run the script, I just have to type `ad` to raycast, then the repository and the branch I want to deploy.

2. Run the apple script:
```bash
osascript raycast.applescript <repository> <branch>
```
prerequisites: a mac

3. Run the autodeployer module:
```bash
go run github.com/psycho-baller/autodeployer <repository> <branch>
```
prerequisites: go installed

4. Run the autodeployer binary:
```bash
./bin/autodeployer
```
prerequisites: nothing, but if you want to rebuild the binary yourself, you will need go installed.
After installing go, you can rebuild the binary by running
```bash
go build -o bin/autodeployer github.com/psycho-baller/autodeployer`
```

## Things you should know before using this script

- By default, the script assumes you don't want to make a new release after someone else has made the previous rc
  - Example: Say someone else makes `rc-2` for a certain branch, you now want to make `rc-3` for the same branch. You will need to modify the default behavior of filtering the tags to only include the ones that you have made
- It also assumes that you are making a minor bump to the version
  - Example: Currently latest version of some branch is `1.2.3`, default behavior will be to make `1.2.4-rc*` if you want to make `1.3.0-rc*` or `2.0.0-rc*`, you will need to modify the default behavior
- It assumes that you do not manually create tags for the release candidates without updating the deployment repo. Always make sure that the deployment repo is up to date with the latest rc tag created
- It assumes you have 1password set up and have the `GHEC_TOKEN` saved in there


## Future improvements

- Tell the user if the deployment failed (easy, semi-quick)
- Add support for making major and patch bumps to the version (easy, quick)
- Handling the case where a certain repo has different files that need to be bumped (like flo) (medium, not so quick)
- How the hell do I make this work for portals?
