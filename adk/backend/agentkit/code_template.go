/*
 * Copyright 2026 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agentkit

const (
	readPythonCodeTemplate = `
import os
import sys

file_path = '{file_path}'
offset = {offset}
limit = {limit}

# Check if file exists
if not os.path.isfile(file_path):
    print('Error: File not found')
    sys.exit(-1)

# Check if file is empty
if os.path.getsize(file_path) == 0:
    print('System reminder: File exists but has empty contents')
    sys.exit(0)

# Read file with offset and limit (offset is 1-indexed, where 1 means the first line)
with open(file_path, 'r') as f:
    collected = []
    for i, line in enumerate(f, 1):
        if i < offset:
            continue
        if i >= offset + limit:
            break
        collected.append(line)

sys.stdout.write("".join(collected))
`
	lsInfoPythonCodeTemplate = `
import os
import json

path = '{path}'

try:
    with os.scandir(path) as it:
        for entry in sorted(it, key=lambda e: e.name):
            result = {{
                'path': entry.name,
                'is_dir': entry.is_dir(follow_symlinks=False)
            }}
            print(json.dumps(result))
except FileNotFoundError:
    pass
except PermissionError:
    pass
`
	writePythonCodeTemplate = `
import os
import base64

file_path = '{file_path}'

# Create parent directory if needed
parent_dir = os.path.dirname(file_path) or '.'
os.makedirs(parent_dir, exist_ok=True)

# Decode and write content
content = base64.b64decode('{content_b64}').decode('utf-8')
with open(file_path, 'w') as f:
    f.write(content)
`
	editPythonCodeTemplate = `
import sys
import base64

# Read file content
with open('{file_path}', 'r') as f:
    text = f.read()

# Decode base64-encoded strings
old = base64.b64decode('{old_b64}').decode('utf-8')
new = base64.b64decode('{new_b64}').decode('utf-8')

# Count occurrences
count = text.count(old)

# Exit with error codes if issues found
if count == 0:
    print(f"Error: String not found in file: '{{old}}'")
    sys.exit(-1)  # String not found
elif count > 1 and not {replace_all}:
    print(f"Error: String '{{old}}' appears multiple times. Use replace_all=True to replace all occurrences.")
    sys.exit(-1)  # Multiple occurrences without replace_all

# Perform replacement
if {replace_all}:
    result = text.replace(old, new)
else:
    result = text.replace(old, new, 1)

# Write back to file
with open('{file_path}', 'w') as f:
    f.write(result)

print(count, end="")
`

	grepPythonCodeTemplate = `
import fnmatch
import json
import subprocess
from pathlib import Path


def build_ripgrep_cmd(file_type, glob_pattern, after_lines, before_lines, pattern, search_path, case_insensitive, multiline):
    cmd = ["rg", "--json"]
    if case_insensitive:
        cmd.append("-i")
    if multiline:
        cmd.extend(["-U", "--multiline-dotall"])
    if file_type:
        cmd.extend(["--type", file_type])
    elif glob_pattern:
        cmd.extend(["--glob", glob_pattern])
    if after_lines and after_lines > 0:
        cmd.extend(["-A", str(after_lines)])
    if before_lines and before_lines > 0:
        cmd.extend(["-B", str(before_lines)])

    cmd.extend(["-e", pattern])
    if search_path:
        cmd.extend(["--", search_path])
    return cmd


def parse_ripgrep_output(output, file_type, glob_pattern):
    responses = []
    if not output:
        return responses

    empty_dict = dict()
    for line in output.split("\n"):
        try:
            data = json.loads(line)
        except json.JSONDecodeError:
            continue

        if data.get("type") not in ("match", "context"):
            continue

        match_data = data.get("data", empty_dict)
        match_path = match_data.get("path", empty_dict).get("text", "")
        lines_data = match_data.get("lines", empty_dict)
        response = dict(
            Path=match_path,
            Line=match_data.get("line_number", 0),
            Content=lines_data.get("text", "").rstrip("\n")
        )

        if file_type and glob_pattern:
            if fnmatch.fnmatch(match_path, glob_pattern) or fnmatch.fnmatch(Path(match_path).name, glob_pattern):
                responses.append(response)
        else:
            responses.append(response)

    return responses


def run_ripgrep(file_type, glob_pattern, after_lines, before_lines, pattern, search_path, case_insensitive, multiline):
    if not search_path:
        return []

    cmd = build_ripgrep_cmd(file_type, glob_pattern, after_lines, before_lines, pattern, search_path, case_insensitive, multiline)

    try:
        result = subprocess.run(cmd, capture_output=True, text=True, check=False)
    except FileNotFoundError:
        raise RuntimeError("ripgrep (rg) is not installed or not in PATH")

    if result.returncode not in (0, 1):
        raise RuntimeError(f"ripgrep failed: {{result.stderr}}")

    return parse_ripgrep_output(result.stdout.strip(), file_type, glob_pattern)


responses = run_ripgrep(
    file_type='{fileType}',
    glob_pattern='{glob}',
    after_lines={afterLines},
    before_lines={beforeLines},
    pattern='{pattern}',
    search_path='{path}',
    case_insensitive={caseInsensitive},
    multiline={enableMultiline}
)
print(json.dumps(responses), end="")
`

	globPythonCodeTemplate = `
import glob
import os
import json
import base64

# Decode base64-encoded parameters
path = base64.b64decode('{path_b64}').decode('utf-8')
pattern = base64.b64decode('{pattern_b64}').decode('utf-8')

os.chdir(path)
matches = sorted(glob.glob(pattern, recursive=True))
results = []
for m in matches:
    stat = os.stat(m)
    result = {{
        'path': m,
        'size': stat.st_size,
        'mtime': stat.st_mtime,
        'is_dir': os.path.isdir(m)
    }}
    results.append(result)
print(json.dumps(results), end="")
`
	executePythonCodeTemplate = `
import sys
import subprocess
import base64

# Decode base64-encoded command
command = base64.b64decode('{command_b64}').decode('utf-8')

try:
    # Execute the command
    result = subprocess.run(command, shell=True, capture_output=True, text=True, check=False)

    # Check for stderr
    if result.stderr:
        print(f"Error executing command: {{result.stderr}}", file=sys.stderr)
        sys.exit(result.returncode if result.returncode != 0 else 1)
    
    # Print stdout
    print(result.stdout, end='')

except Exception as e:
    print(f"Error executing command script: {{e}}", file=sys.stderr)
    sys.exit(1)
`
)
