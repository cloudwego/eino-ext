# Model file references

A media input can reference an existing file hosted by the model provider using `FileID`. Choose exactly one of `URL`, `Base64Data`, and `FileID` per media part. Keep MIME type, image detail, and filename in their existing fields where the target API uses them.

```go
fileID := "provider-file-id"
messages := []*schema.Message{{
    Role: schema.User,
    UserInputMultiContent: []schema.MessageInputPart{{
        Type: schema.ChatMessagePartTypeFileURL,
        File: &schema.MessageInputFile{
            MessagePartCommon: schema.MessagePartCommon{FileID: &fileID},
        },
    }},
}}

agenticMessages := []*schema.AgenticMessage{{
    Role: schema.AgenticRoleTypeUser,
    ContentBlocks: []*schema.ContentBlock{
        schema.NewContentBlock(&schema.UserInputFile{FileID: fileID}),
    },
}}
```

`FileID` is an opaque provider reference. For Gemini, pass the complete `File.URI` returned by the upload API, together with `File.MIMEType`; the file management name (`files/...`) is a different value. The adapter maps the URI to `fileData.fileUri`.

The following table describes the request mappings implemented here. Model, endpoint, project, and account restrictions still apply; a mapping is not a live API compatibility guarantee.

| Component | Media input mappings | Tool-result mappings |
| --- | --- | --- |
| `agenticopenai` (Responses) | Image and file to `file_id` | Image and file to `file_id` |
| `agenticclaude` | Image and file to `source: {type: file, file_id: ...}` | Same mappings |
| `claude` | Image and file to `source: {type: file, file_id: ...}` | Same mappings |
| `agenticark` (Responses) | Image, audio, video, and file to `file_id` | Existing text-only restriction |
| `ark` (Responses) | Image, audio, video, and file to `file_id` | Existing endpoint-specific restrictions |
| `agenticgemini` | Image, audio, video, and file URI to `fileData.fileUri` | Existing text-only restriction |
| `gemini` | Image, audio, video, and file URI to `fileData.fileUri` | URI in function-response file data |

The pinned Ark Chat SDK does not expose file-reference fields. The Chat adapter returns an explicit error for `FileID`; use the Responses adapter for this source. Other components retain their existing capabilities.

The Claude SDK version used here exposes file sources in beta types. The adapters serialize those source objects through the SDK's `param.Override` hook into stable Messages requests; this does not change the endpoint or add a beta header. Availability on Bedrock, Vertex, or compatible third-party endpoints must follow that endpoint's contract.

This change depends on [Eino's media FileID support](https://github.com/cloudwego/eino/pull/1394). The affected modules currently pin its immutable core commit so their builds include the new schema fields.

The application supplies an already available reference. Upload, deletion, renewal, and migration between providers remain application responsibilities.
