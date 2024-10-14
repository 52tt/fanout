# fanout
`fanout` is a `tee`-like tool for replicating stdin to clients.

`fanout` resolves several issues in `<upstream_command> | socat - TCP-LISTEN:<port>,reuseaddr,fork`, including:

- If no client is connected to `socat`, once the pipe is filled up, the upstream command will be blocked.\
  And this issue can not be resolved by adding `tee` command (i.e. using `<upstream_command> | tee >( socat - TCP-LISTEN:<port>,reuseaddr,fork )`).\
  With `fanout`, the upstream command will never be blocked.

- If multiple clients connects to `socat`, `socat` will randomly dispatch received data to one of the clients, instead of replicating to all the clients, so each client receives only a subset of the upstream command outputs.\
  `fanout` replicates stdin input to all the clients, so each client is like directly piped to the upstream command.\

## Usage
```
$ fanout -h
  -h    Show help
  -l string
        Listen address (default "127.0.0.1:2000")

# Examples
<upstream_command> | fanout                       # Listen on default address and port `127.0.0.1:2000`
<upstream_command> | fanout -l 127.0.0.1:2000     # Listen on the given address and port
<upstream_command> | fanout -l /tmp/fanout.sock   # Listen on the given unix domain socket
<upstream_command> | fanout 1>/dev/null           # Suppress stdout printing

# To receive from fanout
- Using nc:     nc 127.0.0.1 2000
- Using ncat:   ncat 127.0.0.1 2000
- Using telnet: telnet 127.0.0.1 2000
- Using socat:  socat TCP:127.0.0.1:2000 -
                socat UNIX-CONNECT:/tmp/fanout.sock -
```
