# Test Audit: Gemini Tool Call ID Passthrough

## Scope

- `components/model/gemini/gemini_test.go`
- `components/model/gemini/tool_call_id_attack_test.go`
- `components/model/agenticgemini/conv_test.go`
- `components/model/agenticgemini/model_test.go`
- `components/model/agenticgemini/tool_call_id_attack_test.go`
- `components/model/agenticgemini/integration_test.go`

## Findings

| Priority | Dimension | Finding | Verdict |
|---|---|---|---|
| Medium | Coverage gap | The multimodal attack test used only a text block, so it did not prove that media parts and call IDs coexist. | Fixed |
| Low | Duplicates | Provider-ID, fallback-ID, and reversed-result tests overlap at the helper level but verify distinct round-trip, streaming, and feature-interaction contracts. | Keep |
| Low | Assertion quality | Dynamic model output uses boundary assertions only to isolate its JSON object; the decoded mapping is asserted exactly. | Keep |

The coverage fix adds an inline image and exact byte assertion at
`components/model/gemini/tool_call_id_attack_test.go:55-86`, plus an exact error
assertion for invalid multimodal input at lines 88-97.

## Six-Dimension Audit

1. **Duplicates:** No strict subset tests. Generate/Stream and classic/agentic pairs are intentional.
2. **Assertion quality:** New deterministic paths use exact equality, length, UUID parsing, and exact errors.
3. **Boilerplate:** No pattern repeats three or more times within the new tests.
4. **Logical grouping:** Conversion, stream, adversarial, and live integration concerns have separate entries.
5. **Semantic value:** Every added case verifies ID identity, pairing, fallback uniqueness, or session reconstruction.
6. **Coverage gaps:** All executable lines added to provider conversion functions are covered locally.

## Coverage

| Module | Package Coverage | Changed Functions |
|---|---:|---|
| `gemini` | 75.4% | `convToolMessageToPart` 100%, `convSchemaMessage` 73.8%, `convFC` 81.8% |
| `agenticgemini` | 77.0% | `convFunctionToolCall` 85.7%, `convFunctionToolResult` 80.0%, `convAgenticFC` 77.8% |

The lower whole-function percentages come from pre-existing branches outside this
diff. All new executable ID mapping and fallback branches are covered, and no
changed function is below the 70% hard floor.

## Result

- High-priority findings: 0
- Medium findings: 1 fixed
- Remaining actionable findings: 0
