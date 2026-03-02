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

# Read file with offset and limit
with open(file_path, 'r') as f:
    lines = f.readlines()

# Apply offset and limit (offset is 1-indexed, where 1 means the first line)
start_idx = offset - 1
end_idx = start_idx + limit
selected_lines = lines[start_idx:end_idx]

# Format with line numbers (1-indexed, starting from offset)
for i, line in enumerate(selected_lines):
    line_num = offset + i
    # Remove trailing newline for formatting, then add it back
    line_content = line.rstrip('\n')
    print(f'{{line_num:6d}}\t{{line_content}}')
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
import sys
import base64

file_path = '{file_path}'

# Check if file already exists (atomic with write)
if os.path.exists(file_path):
    print(f"Error: File '{{file_path}}' already exists", file=sys.stderr)
    sys.exit(-1)

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

print(count)
`

	grepPythonCodeTemplate = `
import fnmatch
import json
import subprocess
from pathlib import Path


fileType = '{fileType}'
glob = '{glob}'
afterLines = {afterLines}
beforeLines = {beforeLines}
pattern = '{pattern}'
path = '{path}'

cmd = ["rg", "--json"]
if {caseInsensitive}:
    cmd.append("-i")

if {enableMultiline}:
    cmd.extend(["-U", "--multiline-dotall"])

if fileType:
    cmd.extend(["--type", fileType])
elif glob:
    cmd.extend(["--glob", glob])

if afterLines and afterLines > 0:
    cmd.extend(["-A", str(afterLines)])

if beforeLines and beforeLines > 0:
    cmd.extend(["-B", str(beforeLines)])
cmd.append(pattern)
if path:
    cmd.append(path)
    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            check=False
        )
        if result.returncode not in (0, 1):
            raise RuntimeError(f"ripgrep failed: {{result.stderr}}")

        responses = []
        empty_dict = dict()
        output = result.stdout.strip()
        if output:
            for line in output.split("\n"):
                try:
                    data = json.loads(line)
                    if data.get("type") == "match" or data.get("type") == "context":
                        match_data = data.get("data", empty_dict)
                        path = match_data.get("path", empty_dict).get("text", "")
                        lines = match_data.get("lines", empty_dict)
                        response = dict(
                            Path=path,
                            Line=match_data.get("line_number", 0),
                            Content=lines.get("text", "").rstrip("\n")
                        )
                        if fileType and glob:
                            if fnmatch.fnmatch(path, glob) or fnmatch.fnmatch(Path(path).name, glob):
                                responses.append(response)
                        else:
                            responses.append(response)
                except json.JSONDecodeError:
                    continue
        print(json.dumps(responses))
    except FileNotFoundError:
        raise RuntimeError("ripgrep (rg) is not installed or not in PATH")
else:
    print("[]")
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
for m in matches:
    stat = os.stat(m)
    result = {{
        'path': m,
        'size': stat.st_size,
        'mtime': stat.st_mtime,
        'is_dir': os.path.isdir(m)
    }}
    print(json.dumps(result))
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
