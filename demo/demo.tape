# The README demo: flags, sessions, models, MCP servers and flag values, completed from a made-up ~/.claude.
#
# The settings come first, as the recorder ignores any Set directive that is not at the top of the file.
Source demo/common.tape

Output img/demo.gif

# Every scene starts at an empty prompt, waited for rather than slept on: keys typed while the shell is still
# busy are lost or land in the next command line.
Hide
Type "source demo/zshrc && clear"
Enter
Wait+Line@1m /^\x{276f}\s*$/
Show

Sleep 500ms

# the flags, coloured by whether they take a value, then the sessions of the project: the named ones by name,
# the others by id, all with what they were about
Type "claude --re"
Sleep 150ms
Tab
Sleep 750ms
Tab@250ms 4
Sleep 400ms
Enter
Sleep 400ms
Tab
Sleep 1000ms
Tab@350ms 3
Sleep 250ms
Enter
Sleep 1000ms

# a Ctrl+C in a completion menu can be swallowed, so the line is also cleared
Hide
Ctrl+C
Sleep 500ms
Ctrl+U
Type "clear"
Enter
Wait+Line /^\x{276f}\s*$/
Show

# the model aliases from the help, and the models of the account
Type "claude --model "
Sleep 250ms
Tab
Sleep 1000ms
Tab@250ms 3
Sleep 400ms
Enter
Sleep 750ms

Hide
Ctrl+C
Sleep 500ms
Ctrl+U
Type "clear"
Enter
Wait+Line /^\x{276f}\s*$/
Show

# the MCP servers, and the pull requests linked to the sessions
Type "claude mcp get "
Sleep 250ms
Tab
Sleep 1000ms

Hide
Ctrl+C
Sleep 500ms
Ctrl+U
Type "clear"
Enter
Wait+Line /^\x{276f}\s*$/
Show

Type "claude --from-pr "
Sleep 250ms
Tab
Sleep 1000ms

Hide
Ctrl+C
Sleep 500ms
Ctrl+U
Type "clear"
Enter
Wait+Line /^\x{276f}\s*$/
Show

# the values a flag allows
Type "claude --perm"
Sleep 150ms
Tab
Sleep 750ms
Tab@250ms 2
Sleep 400ms
Enter
Sleep 400ms
Tab
Sleep 750ms
Tab@250ms 5
Sleep 400ms
Enter
Sleep 1250ms
