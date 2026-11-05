# Third-party notices

Maccy-MyGo is an unofficial Go/MyGo rewrite of [Maccy](https://github.com/p0deje/Maccy). It is not an official Maccy release.

| Component | Use | License notice |
| --- | --- | --- |
| Maccy, Alex Rodionov, reference commit `f0b571e32e4031d7b91ba811f50d0f33e8e5548b` | Behavioral reference, adapted logic, menu bar template assets; application icon attribution retained conservatively | [MIT](resources/licenses/Maccy.txt) |
| Fuse-Swift 1.4.0, Kirollos Risk, revision `26ba868691b2d8b7bf2b1322951eb591be70ccca` | Adapted Bitap fuzzy search | [MIT](resources/licenses/Fuse-Swift.txt) |
| Lucide / Feather | Toolbar SVG paths | [ISC / MIT](resources/licenses/Lucide.txt) |
| MyGo v0.7.4 | Native desktop UI and platform integrations | [MIT](resources/licenses/github.com_egoist_mygo-LICENSE.txt) |

The licenses of the Go runtime and linked Go modules are retained in [`resources/licenses`](resources/licenses). MyGo copies this directory into application bundles so binary distributions retain the same notices. A Go rewrite does not remove the license obligations of reused code or assets.

The `reference-Maccy` and `reference-MyGo` directories are local comparison checkouts. They are not included in the published repository or required to build it; the revision links above identify the references.
