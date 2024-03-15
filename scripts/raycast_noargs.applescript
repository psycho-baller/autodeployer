#!/usr/bin/osascript

# Required parameters:
# @raycast.schemaVersion 1
# @raycast.title Github Autodeployer
# @raycast.mode silent

# Optional parameters:
# @raycast.packageName autodeployer_noargs
# @raycast.icon ../assets/rocket.png

# Documentation:
# @raycast.description takes in two parameters, the first is the repository and second one is the branch name. The script will create a new release and automatically deploy the release by running the workflow.
# @raycast.author Rami Maalouf
# @raycast.authorURL https://ramimaalouf.tech

do shell script "./bin/autodeployer"
