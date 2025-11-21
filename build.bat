:: 设置中文环境
chcp 65001

@echo off


echo. 开始构建 proxy.exe
go build -o proxy.exe cmd/proxy/proxy.go
echo. 开始构建 install.exe
go build -o install.exe cmd/install/main.go
echo. 构建结束,请按任意键退出
pause
