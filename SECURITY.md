# Security

Please do **not** open a public issue for a vulnerability.

Report privately through GitHub: **Security → Report a vulnerability** on this repository. If that form is unavailable, email the address on my GitHub profile.

I will acknowledge the report and say whether it is in scope.

This project is local-first tooling. `repo-context serve` binds to `127.0.0.1` by default on purpose — do not expose it on a public interface without reviewing `/api/source` and the rest of the read API.
