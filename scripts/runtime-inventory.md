# Runtime inventory

`scripts/check-runtime.py` checks the executables declared by the current
Dockerfile. It does not install software, run scans, or modify AntiFox's tool
registry. Requires Python 3 (already included in the image).

Run against a fresh, isolated container, not the production service:

```sh
docker run --rm -i --network none --read-only --cap-drop ALL \
  --security-opt no-new-privileges --tmpfs /tmp \
  --entrypoint python3 ghcr.io/chathurangabw/antifox:v2.1.1 \
  - --image-ref ghcr.io/chathurangabw/antifox:v2.1.1 \
  < scripts/check-runtime.py > runtime-inventory.json
```

Use an immutable image digest for release evidence. `--image-ref` is a label
supplied by the operator; independently capture `docker image inspect` to prove
which image was tested. Repeat for each supported architecture.

## Status meanings

- **Missing:** executable not found on the container's PATH.
- **Present:** executable found; no functional claim is made.
- **Identity check passed:** a fixed version command exited successfully.
- **Dependency checks:** AntiFox executable, skills directory, Python DOCX
  creation in memory, and Chromium file presence.

Exit code 1 means a required presence, identity, or dependency check failed.
No default wordlist location is assumed. Pass `--resource-path /path/to/file`
as often as needed to check specific optional files; their contents are never
read. Arbitrary PATH additions or production volumes can change results; the
isolated image check does not measure those additions.

These checks do not prove scanning correctness, browser launch, proxy routing,
permission isolation, process cancellation, or successful operation against a
target. Installed versions may differ from examples written for newer versions.

## Recorded baseline

The initial v2.1.1 amd64 image check found all 28 listed executables and passed
all 14 fixed version checks. A later check of the running service found
`/app/data/i4/common.txt` and `/app/data/i79/common.txt` in its data volume;
these paths do not establish that either file is bundled with the image, and
their contents were not inspected. This is an observed baseline, not a promise
about future tags or deployments.
