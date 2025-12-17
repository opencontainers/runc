## Release Cadence and Support Policy ##

This document describes the release cadence for runc as well as outlining the
support policy for old release branches. Historically, despite runc being the
most widely used Linux container runtime, our release schedule has been very
ad-hoc and has resulted in very long periods of time between minor releases,
causing issues for downstreams that wanted particular features.

### Semantic Versioning ###

runc uses [Semantic Versioning][semver] for releases. However, our
compatibility policy only applies to the runc binary. We will make a
best-effort attempt to reduce the impact to users that make direct use of the
Go packages prefixed with `github.com/opencontainers/runc`, but we do not
formally guarantee that API compatibility will be preserved.

[semver]: https://semver.org/spec/v2.0.0.html

### Release Cadence ###

[new-issue]: https://github.com/opencontainers/runc/issues/new/choose

runc follows a 6-month minor version release schedule, with the aim of releases
happening at the end of April and October each year.

The first release candidate will be created 2 months before the planned release
date (i.e. the end of February and August, respectively), at which point the
release branch will be created and will enter a feature freeze. No new features
will be merged into the release branch, and large features being developed
immediately before the feature freeze may have their merge delayed so as to not
be included in the next release. Most releases will have two or three release
candidates, but this may change depending on the circumstances of the release
at the time.

If a last-minute critical issue is discovered, the release may be delayed.
However, the following release will still go according to schedule (except in
the exceptionally unlikely scenario where the delay is 4-6 months long, in
which case the next release is moved forward to when the subsequent release
would have been).

Here is a hypothetical release timeline to see how this works in practice:

| Date       | Release      | Notes |
| ---------- | ------------ | ----- |
| 200X-02-28 | `1.3.0-rc.1` | `release-1.3` branch created, feature freeze. |
| 200X-03-12 | `1.3.0-rc.2` | |
| 200X-03-25 | `1.3.0-rc.3` | |
| 200X-04-30 | `1.3.0`      | `1.3` release published. |
| 200X-05-10 | `1.3.1`      | |
| 200X-06-21 | `1.3.2`      | |
| 200X-06-25 | `1.3.3`      | |
| 200X-07-02 | `1.3.4`      | |
| 200X-08-28 | `1.4.0-rc.1` | `release-1.4` branch created, feature freeze. |
| 200X-09-15 | `1.3.5`      | Patch releases in other release branches have no impact on the new release branch. |
| 200X-09-21 | `1.4.0-rc.2` | |
| 200X-10-31 | `1.4.0`      | `1.4` release published. |
| 200X-11-10 | `1.4.1`      | |
| 200X-12-25 | `1.4.2`      | |

(And so on for the next year.)

### Support Policy ###

In order to ease the transition between minor runc releases, previous minor
release branches of runc will be maintained for some time after the newest
minor release is published. In the following text, `latest` refers to the
latest minor (non-release-candidate) runc release published; `latest-1` is the
previous minor release branch; and `latest-2` is the minor release branch
before `latest-1`. For example, if `latest` is `1.4.0` then `latest-1` is
`1.3.z` and `latest-2` is `1.2.z`.

 * Once `latest` is released, new features will no longer be merged into
   `latest` and only bug and security fixes will be backported, though we will
   be fairly liberal with what kinds of bugs will considered candidates for
   backporting.

 * `latest-1` will only receive security fixes and significant bug fixes (what
   bug fixes are "significant" are down to the maintainer's judgement, but
   maintainers should err on the side of reducing the number of backports at
   this stage). At this stage, users of `latest-1` are encouraged to start
   planning the migration to the `latest` release of runc (as well as reporting
   any issues they may find).

 * `latest-2` will only receive high severity security fixes (i.e. CVEs that
   have been assessed as having a CVSS score of 7.0 or higher). At this stage,
   users still using `latest-2` would be strongly encouraged to upgrade to
   either `latest` or `latest-1`.

 * Any older releases will no longer receive any updates, and users are
   encouraged to upgrade in the strongest possible terms, as they will not
   receive any security fixes regardless of severity or impact.

This policy only applies to minor releases of runc with major version `1`. If
there is a runc `2.0` release in the future, this document will be updated to
reflect the necessary changes to the support policy for the `1.y` major release
branch of runc.
