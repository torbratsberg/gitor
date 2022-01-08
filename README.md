# Gitor

Manage your git repos on a remote server.

Kind of a CLIified bare bone version of github. Kinda...

## Functionality

- See all the repositories on your server.
- See info about a specific repository.
- Add new repositories.
- Delete repositories.

## Usage

- See `$ gitor_client --help` for all commands.
- Run `$ gitor_server` to start the server.

### Installation

- `$ git clone https://github.com/torbratsberg/gitor`
- `$ cd gitor`
- Run `$ go install` in both the `/server` and `/client` directory.

### Configuration / setup

**Note**: All values needs to be strings

- Copy `config-server.example.yml` to `$HOME/.config/gitor/config-server.yml` on your server.
- Copy `config-client.example.yml` to `$HOME/.config/gitor/config-client.yml` on your computer.

#### Client config file values

```yaml
remoteserver:
  address: IP or domain for your remote server.
  port: The port gitor_server is running on.
  token: Token used for authentication. Can include all characters.
```

#### Server config file values

```yaml
paths:
  repositories: The folder where all your repositories will be placed. Remeber to create this directory before starting server.
server:
  port: Port to run server on.
  tokenwhitelist:
    - A list of all accepted/whitelisted tokens.
  sshport: Which port SSH is on.
  address: IP or domain for your remote server.
  user: The user Git should use. Using "git" is probably a good idea.

```
