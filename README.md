# FXMacroData for FastJSON

Use complete FXMacroData responses through native `valyala/fastjson` values. This independent companion is not an upstream endorsement.

```sh
go get github.com/fxmacrodata/fastjson-fxmacrodata@v0.1.0
```

```go
import fxmd "github.com/fxmacrodata/fastjson-fxmacrodata"

// responseJSON comes from your own current FXMacroData SDK/client.
doc, err := fxmd.ParseJSON(responseJSON)
if err != nil { return err }
response, err := doc.Native()
if err != nil { return err }
selected, err := doc.NativeAt("/data/0") // choose your actual JSON Pointer
if err != nil { return err }
consumeNative(response, selected)
```

`Native()` returns a new complete native tree; `NativeAt()` selects a complete value by RFC 6901 JSON Pointer. All fields, nested values, revisions, provenance, units, clocks, unknown-time flags, explicit nulls and pagination metadata remain available. No parser instance is reused: later calls and native edits cannot invalidate or change earlier views or the original document. Each native tree is independently mutable and follows the upstream library's normal concurrency rules.

`Document.Raw()` returns an independent copy of the exact original response bytes. `JSONAt()` retains every selected field and numeric spelling while normalizing whitespace, escaping and object order. `NumberTextAt()` exposes the exact numeric lexeme. Native `Value.String()`/`MarshalTo()` retain number text without implicit float64 conversion. Use numeric conversion methods only when you explicitly want their native range/precision behavior. Native edits are derived values and never rewrite original observations. No dates or missing values are guessed.

`Parse` accepts JSON-serializable SDK models; `ParseJSON` also preserves unknown fields and original bytes that typed models may omit. Values already rounded by the caller cannot be reconstructed. Empty values and explicit nulls are supported; absent pointers return errors rather than fabricated zeroes. Duplicate keys, malformed JSON, invalid UTF-8, excess depth/value count and oversized input/views are rejected with payload-free errors. Limits: 4 MiB response, 8 MiB view, 64 nested containers, 50,000 JSON values and 4,096 UTF-8 pointer bytes. Original JSON remains available if a selected native representation fails.

The local MIT-licensed adapter and MIT-licensed native library need no hosted plan, account, card or infrastructure. Customers supply their own client, credentials and entitlements outside this package; always-free USD catalogue, macro history and calendar can be obtained through that client. There is no key/environment access, HTTP transport, endpoint/parameter allowlist, automatic pagination, live fallback or data storage. Future JSON response families can use the same interface. Media/stream/MCP execution belongs to the caller's client or renderer.

[FXMacroData](https://fxmacrodata.com/?utm_source=github&utm_medium=integration&utm_campaign=fastjson&utm_content=readme) · [API documentation](https://fxmacrodata.com/docs/?utm_source=github&utm_medium=integration&utm_campaign=fastjson&utm_content=docs)

See [CAPABILITIES.md](CAPABILITIES.md). Native fixture tests cover exact numbers, null/missing distinctions, every JSON root shape, complete response families, independent native mutations/lifetimes, pointers, Unicode and bounds. `go run ./examples/inspect` uses a fictional fixture only. Tests do not certify live endpoints/authentication, real data or customer installation.
