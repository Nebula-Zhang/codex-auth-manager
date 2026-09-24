# Codex Auth Manager

A small cross-platform command-line tool for safely managing multiple Codex `auth*.json` files.

一个用于安全管理多个 Codex `auth*.json` 文件的轻量级跨平台命令行工具。

[English](#english) | [中文](#中文)

---

## English

### Features

- Scans `auth*.json` files in a configured `.codex` directory.
- Switches another auth file to the standard `auth.json` name.
- Renames any listed auth file, including the active `auth.json`.
- Saves the `.codex` directory in `config.json` beside the executable.
- Provides standalone binaries for Windows x86-64, Linux x86-64, and Linux ARM64/aarch64.
- Never reads, parses, or prints the contents of an auth file.
- Never overwrites an existing file.
- Shows planned changes and asks for confirmation before switching or renaming.
- Attempts to restore the original `auth.json` if the second step of a switch fails.

### Main menu

```text
Codex Auth Manager
Codex directory: C:\Users\user\.codex

Auth files:
1. auth.json [ACTIVE]
2. auth_personal.json
3. auth_work.json

S - Switch auth file
R - Rename an auth file
P - Change .codex path
Q - Quit
Select action:
```

| Action | Description |
| --- | --- |
| `S` | Select an auth file and activate it as `auth.json`. If `auth.json` already exists, the tool first asks for a name under which to save it. |
| `R` | Select and rename any auth file without activating another file. |
| `P` | Change the saved absolute path to the `.codex` directory. |
| `Q` | Exit the program. |

Enter `B` when offered to cancel the current operation and return to the main menu.

### Switch example

Suppose `auth.json` is active and you want to activate `auth_work.json`:

```text
Select action: S
Enter an auth file number, or B to go back: 2
Enter a name to save the current auth.json, or B to go back: personal

Planned changes:

auth.json -> auth_personal.json
auth_work.json -> auth.json

Continue? (Y/N): Y
```

The two rename operations are kept separate internally. If activating the selected file fails after the current `auth.json` has been saved, the tool attempts to restore the original `auth.json` automatically.

### Rename example

`R` can rename either the active file or any other listed auth file:

```text
Select action: R
Enter an auth file number, or B to go back: 3
Enter a new name for auth_old.json, or B to go back: archive

Planned change:

auth_old.json -> auth_archive.json

Continue? (Y/N): Y
```

You may enter either `archive` or `auth_archive.json`. Both produce `auth_archive.json`.

New names may contain only:

- ASCII letters: `A-Z`, `a-z`
- Numbers: `0-9`
- Underscores: `_`
- Hyphens: `-`

### Installation and usage

Download the binary for your operating system from the GitHub Releases page. Go is not required to run a compiled binary.

#### Windows x86-64

Download `codex-auth-manager-windows-amd64.exe`, place it in a user-writable directory, then double-click it or run:

```powershell
.\codex-auth-manager-windows-amd64.exe
```

#### Linux x86-64

Download `codex-auth-manager-linux-amd64`, grant execute permission once, then run:

```sh
chmod +x codex-auth-manager-linux-amd64
./codex-auth-manager-linux-amd64
```

#### Linux ARM64/aarch64

Download `codex-auth-manager-linux-arm64`, grant execute permission once, then run:

```sh
chmod +x codex-auth-manager-linux-arm64
./codex-auth-manager-linux-arm64
```

On first run, enter the absolute path to your `.codex` directory:

```text
Windows: C:\Users\user\.codex
Linux:   /home/user/.codex
```

The program creates `config.json` beside the executable. Keep the executable in a directory where the current user has write access. Do not place it in a protected directory such as `Program Files` or `/usr/bin`.

### File matching and safety

- Only regular files in the top level of the configured directory are listed.
- Existing filenames are matched using the lowercase pattern `auth*.json`.
- Files such as `auth.json.backup` and `other_auth.json` are ignored.
- New filenames always use the standard `auth_<name>.json` format.
- Existing destination files are never replaced.
- Auth file contents are never inspected or displayed.
- A newly started Codex session may be required after switching accounts.

### Build from source

Go 1.22 or newer is required only for development and compilation.

Run tests:

```sh
go test ./...
go vet ./...
```

Build for Windows x86-64 from PowerShell:

```powershell
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags="-s -w" -o codex-auth-manager-windows-amd64.exe .
```

Build for Linux x86-64 from PowerShell:

```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags="-s -w" -o codex-auth-manager-linux-amd64 .
```

Build for Linux ARM64/aarch64 from PowerShell:

```powershell
$env:GOOS = "linux"
$env:GOARCH = "arm64"
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags="-s -w" -o codex-auth-manager-linux-arm64 .
```

---

## 中文

### 功能特点

- 扫描所配置 `.codex` 目录中的 `auth*.json` 文件。
- 将选中的认证文件切换为标准的 `auth.json`。
- 可以重命名列表中的任意认证文件，包括当前使用的 `auth.json`。
- 将 `.codex` 路径保存在可执行文件旁边的 `config.json` 中。
- 提供可独立运行的 Windows x86-64、Linux x86-64 和 Linux ARM64/aarch64 程序。
- 不读取、不解析、也不打印认证文件的内容。
- 绝不覆盖已有文件。
- 切换或重命名前会展示变更内容，并要求用户确认。
- 如果切换的第二步失败，会尝试自动恢复原来的 `auth.json`。

### 主菜单

```text
Codex Auth Manager
Codex directory: C:\Users\user\.codex

Auth files:
1. auth.json [ACTIVE]
2. auth_personal.json
3. auth_work.json

S - Switch auth file
R - Rename an auth file
P - Change .codex path
Q - Quit
Select action:
```

| 操作 | 说明 |
| --- | --- |
| `S` | 选择一个认证文件，并将其激活为 `auth.json`。如果当前已有 `auth.json`，程序会先要求为当前文件输入一个保存名称。 |
| `R` | 选择并重命名任意认证文件，不会同时激活其他文件。 |
| `P` | 修改已保存的 `.codex` 文件夹绝对路径。 |
| `Q` | 退出程序。 |

在操作过程中看到相应提示时，可以输入 `B` 取消当前操作并返回主菜单。

### 切换示例

假设 `auth.json` 正在使用，现在希望启用 `auth_work.json`：

```text
Select action: S
Enter an auth file number, or B to go back: 2
Enter a name to save the current auth.json, or B to go back: personal

Planned changes:

auth.json -> auth_personal.json
auth_work.json -> auth.json

Continue? (Y/N): Y
```

程序内部仍将两次重命名作为两个独立步骤处理。如果保存当前 `auth.json` 后，激活所选文件失败，程序会尝试自动恢复原来的 `auth.json`。

### 重命名示例

`R` 既可以重命名当前文件，也可以重命名列表中的其他认证文件：

```text
Select action: R
Enter an auth file number, or B to go back: 3
Enter a new name for auth_old.json, or B to go back: archive

Planned change:

auth_old.json -> auth_archive.json

Continue? (Y/N): Y
```

输入 `archive` 或完整的 `auth_archive.json` 均可，最终名称都是 `auth_archive.json`。

新名称仅允许使用：

- 英文字母：`A-Z`、`a-z`
- 数字：`0-9`
- 下划线：`_`
- 短横线：`-`

### 安装与使用

从 GitHub Releases 页面下载对应操作系统的程序。运行编译完成的程序不需要安装 Go。

#### Windows x86-64

下载 `codex-auth-manager-windows-amd64.exe`，将其放入当前用户有写入权限的目录，然后双击运行，或在 PowerShell 中执行：

```powershell
.\codex-auth-manager-windows-amd64.exe
```

#### Linux x86-64

下载 `codex-auth-manager-linux-amd64`，首次运行前赋予执行权限：

```sh
chmod +x codex-auth-manager-linux-amd64
./codex-auth-manager-linux-amd64
```

#### Linux ARM64/aarch64

下载 `codex-auth-manager-linux-arm64`，首次运行前赋予执行权限：

```sh
chmod +x codex-auth-manager-linux-arm64
./codex-auth-manager-linux-arm64
```

第一次启动时，需要输入 `.codex` 文件夹的绝对路径：

```text
Windows: C:\Users\user\.codex
Linux:   /home/user/.codex
```

程序会在可执行文件旁边创建 `config.json`。因此，请将程序放在当前用户可以写入的目录中，不要放到 `Program Files`、`/usr/bin` 等受保护目录。

### 文件匹配与安全规则

- 仅扫描所配置目录顶层的普通文件。
- 现有文件使用小写的 `auth*.json` 规则匹配。
- `auth.json.backup`、`other_auth.json` 等文件不会被列出。
- 工具创建的新名称始终采用 `auth_<名称>.json` 格式。
- 如果目标名称已经存在，程序会拒绝操作，不会覆盖文件。
- 程序不会检查或显示认证文件的内容。
- 切换账号后，可能需要重新启动一个 Codex 会话才能使用新的认证。

### 从源码构建

只有开发和编译时才需要 Go 1.22 或更高版本。

运行测试：

```sh
go test ./...
go vet ./...
```

在 PowerShell 中构建 Windows x86-64 版本：

```powershell
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags="-s -w" -o codex-auth-manager-windows-amd64.exe .
```

在 PowerShell 中构建 Linux x86-64 版本：

```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags="-s -w" -o codex-auth-manager-linux-amd64 .
```

在 PowerShell 中构建 Linux ARM64/aarch64 版本：

```powershell
$env:GOOS = "linux"
$env:GOARCH = "arm64"
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags="-s -w" -o codex-auth-manager-linux-arm64 .
```
