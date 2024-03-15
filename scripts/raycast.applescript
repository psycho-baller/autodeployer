#!/usr/bin/osascript

# Required parameters:
# @raycast.schemaVersion 1
# @raycast.title Github Autodeployer
# @raycast.mode silent

# Optional parameters:
# @raycast.packageName autodeployer
# @raycast.icon ../assets/rocket.png
# @raycast.argument1 { "type": "text", "placeholder": "repository" }
# @raycast.argument2 { "type": "text", "placeholder": "branch" }
# @raycast.argument3 { "type": "text", "placeholder": "old tag", optional: true }

# Documentation:
# @raycast.description takes in two parameters, the first is the repository and second one is the branch name. The script will create a new release and automatically deploy the release by running the workflow.
# @raycast.author Rami Maalouf
# @raycast.authorURL https://ramimaalouf.tech

on run argv
    if (count of argv) = 2 then
        set repo to item 1 of argv
        set branch to item 2 of argv
        do shell script "./bin/autodeployer " & repo & " " & branch
    else if (count of argv) = 3 then
        set repo to item 1 of argv
        set branch to item 2 of argv
        set oldTag to item 3 of argv
        do shell script "./bin/autodeployer " & repo & " " & branch & " " & oldTag
    else
        display dialog "Invalid number of arguments. Please provide 2 or 3 arguments."
    end if
end run