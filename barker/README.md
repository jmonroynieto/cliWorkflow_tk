## barker

CLI utility to notify about the creation of a specified file.

#### Security

The security of your password is your responsibility. To avoid text saves of your password, please avoid writing the export statement in `.bashrc` or `.bash_profile`. Additionally, HISTIGNORE can be used to avoid saving the command to your shell's history. 
```shell
#add to ~/.bashrc (non-interactive) or ~/.bash_profile
export HISTIGNORE="pwd:ls:*SMTP_PASSWORD*:[ \t]*" 
```