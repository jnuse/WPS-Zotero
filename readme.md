# WPS-Zotero (Go 版本)

本项目是 [WPS-Zotero](https://github.com/tankwyn/WPS-Zotero) 的 Go 语言重构版本。原项目使用 Python，本项目将其部分功能改用 Go 实现，并对部分文本进行了中文化。

## 特性

-   使用 Go 语言重写了代理服务器和安装程序，提高了性能和跨平台兼容性。
-   简化了安装流程，通过批处理脚本一键完成构建和安装。
-   将代理服务器的启动集成到安装脚本中，避免了 WPS 弹出不安全的警告。
-   提供了卸载脚本，可以轻松地从系统中移除插件。

## 使用流程

### 1. 构建

双击运行 `build.bat` 脚本。该脚本会编译 Go 源代码，生成 `proxy.exe` 和 `install.exe` 两个可执行文件。

### 2. 安装

双击运行 `install.bat` 脚本。该脚本会执行以下操作：
-   自动将插件文件复制到 WPS 的插件目录。
-   修改 WPS 的配置文件以加载插件。
-   在后台启动 `proxy.exe` 代理服务器，用于 WPS 插件和 Zotero 客户端之间的通信。

### 3. 卸载

当您关闭 WPS 后，可以双击运行 `uninstall.bat` 脚本。该脚本会：
-   关闭正在运行的 `proxy.exe` 进程。
-   从 WPS 插件目录中删除插件文件。
-   清理 WPS 的配置文件。

## 注意事项

-   本项目将原项目中由 JavaScript 唤起代理服务器的逻辑移到了 `install.bat` 流程里，这是为了避免 WPS 报告不安全脚本的警告。
-   如果您需要长期使用此插件，可以将 `proxy.exe` 注册为系统服务，使其能够常驻后台运行。

## 目录结构

```
.
├── cmd/                # Go 源码目录
│   ├── install/        # 安装程序源码
│   └── proxy/          # 代理服务器源码
├── wpsjs/              # WPS 加载项的前端文件 (HTML, JS, CSS)
├── build.bat           # 构建脚本
├── install.bat         # 安装脚本
├── uninstall.bat       # 卸载脚本
├── go.mod              # Go 模块文件
└── README.md           # 本文档
