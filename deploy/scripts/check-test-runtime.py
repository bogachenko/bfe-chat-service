#!/usr/bin/env python3
"""Read-only test VPS checks. Stdout contains only fixed nonsecret markers."""
import argparse
import re
import sys

def blocks(source):
    """Parse nginx brace nesting while ignoring comments and quoted braces."""
    tokens = re.findall(r'(?:"(?:\\.|[^"\\])*"|\x27(?:\\.|[^\x27\\])*\x27|\# [^\n]*|\#[^\n]*|[{};]|[^{};\s]+)', source)
    stack = []
    result = []
    statement = []
    for token in tokens:
        if token.startswith("#"):
            continue
        if token == "{":
            name = " ".join(statement).strip()
            if not name:
                raise ValueError("invalid nginx block")
            stack.append([name, [], []])
            statement.clear()
        elif token == "}":
            if not stack or statement:
                raise ValueError("unbalanced nginx config")
            name, directives, children = stack.pop()
            item = (name, directives, children)
            if stack:
                stack[-1][2].append(item)
            else:
                result.append(item)
        elif token == ";":
            if not stack or not statement:
                statement.clear()
                continue
            stack[-1][1].append(" ".join(statement))
            statement.clear()
        else:
            statement.append(token.strip('"\x27'))
    if stack:
        raise ValueError("unbalanced nginx config")
    return result

def descendants(nodes, label):
    for node in nodes:
        if node[0].split()[0] == label:
            yield node
        yield from descendants(node[2], label)

def nginx_ok(config, host):
    candidates = []
    for server in descendants(blocks(config), "server"):
        directives = server[1]
        names = [v.split()[1:] for v in directives if v.startswith("server_name ")]
        listen = [v for v in directives if v.startswith("listen ")]
        if not any(host in values for values in names):
            continue
        if not any(re.search(r'(^|\s)(443|[^\s:]+:443)(\s|$)', v) and "ssl" in v.split() for v in listen):
            continue
        locations = [x for x in descendants(server[2], "location")]
        routes = set()
        for loc in locations:
            parts = loc[0].split()
            if len(parts) != 2 or parts[1] not in ("/ws", "/v1/"):
                continue
            targets = [d.split()[1] for d in loc[1] if d.startswith("proxy_pass ") and len(d.split()) == 2]
            if targets == ["http://127.0.0.1:8081"]:
                routes.add(parts[1])
        if routes == {"/ws", "/v1/"}:
            candidates.append(server)
    # Reject ambiguous or multiple host matches.
    return len(candidates) == 1

def port_owned(ss_output, main_pid):
    if not main_pid.isdecimal() or int(main_pid) <= 1:
        return False
    rows = [line for line in ss_output.splitlines() if ":8081" in line]
    if not rows:
        return True  # Before first deployment, port can be free.
    if len(rows) != 1:
        return False
    line = rows[0]
    return bool(re.search(r'\b127\.0\.0\.1:8081\b', line) and
                re.search(r'pid=' + re.escape(main_pid) + r'[,)]', line))

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--nginx", required=True)
    parser.add_argument("--host", required=True)
    parser.add_argument("--ss", required=True)
    parser.add_argument("--pid", required=True)
    a = parser.parse_args()
    if not nginx_ok(open(a.nginx, encoding="utf-8").read(), a.host):
        raise ValueError("HTTPS vhost and Chat upstream are not in the same valid server block")
    if not port_owned(open(a.ss, encoding="utf-8").read(), a.pid):
        raise ValueError("port 8081 is not owned by the Chat service MainPID")
    print("NGINX_CHAT_VHOST=PASS")
    print("PORT_8081_OWNER=PASS")

if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError) as error:
        print(f"preflight: {error}", file=sys.stderr)
        sys.exit(1)
