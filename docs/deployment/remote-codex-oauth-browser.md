# Server-side Codex OAuth browser runtime

QAPI completes the CLIProxyAPI Codex OAuth flow entirely on the server. The
administrator's browser only displays screenshots and forwards input events to
the temporary server-side browser session.

## Runtime layout

The production deployment uses these paths:

```text
/opt/new-api/remote-browser/chrome-linux64/chrome
/opt/new-api/remote-browser/full-deps/usr/bin/Xvfb-qapi
/opt/new-api/remote-browser/full-deps/usr/bin/xkbcomp
/opt/new-api/remote-browser/sessions/
```

- Chrome for Testing: `150.0.7871.115`
- Patched `Xvfb-qapi` SHA-256:
  `f047ef75089f8284fc328a3e123f64d677094d3cb449335c022c0958083b1d65`
- Session directory owner/mode: `newapi:newapi`, `0700`
- Chrome DevTools ports bind only to `127.0.0.1`.
- X11 TCP listening is disabled.

## Why Xvfb is required

OpenAI's security verification consistently classifies Chrome headless mode as
automation. Running the same full Chrome build on an isolated Xvfb display
passes the verification and reaches the normal OpenAI sign-in page while the
entire OAuth flow remains on the server.

Ubuntu's Xvfb build uses the compiled path `/usr/bin/xkbcomp`. The production
host intentionally does not install system packages, so the deployed Xvfb copy
changes its embedded XKB binary directory from `/usr/bin` to `.` and runs with
its working directory set to the extracted `usr/bin` directory. No files under
the host's `/usr` tree are changed.

The equivalent deterministic patch is:

```python
from pathlib import Path

path = Path("Xvfb-qapi")
data = path.read_bytes()
old = b"/usr/bin\0"
new = b".\0" + b"\0" * (len(old) - 2)
if data.count(old) != 1:
    raise RuntimeError("unexpected Xvfb binary layout")
path.write_bytes(data.replace(old, new))
```

## Lifecycle and security

Only one remote login session may run at a time. QAPI allocates a private CDP
port and an unused X display, creates a mode-`0700` Chrome profile, and binds
the session to the initiating administrator and OAuth state. Success, failure,
cancellation, expiry, or an unexpected Chrome/Xvfb exit terminates both
processes and removes the temporary profile.

Passwords, verification codes, screenshots and page content are not written to
QAPI logs. Browser process logs live inside the temporary profile and are
deleted with the session.
