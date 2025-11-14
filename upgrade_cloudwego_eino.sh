#!/bin/bash

# 依赖包名称
DEPENDENCY="github.com/cloudwego/eino"
# 指定的版本号或分支（如果未提供，则更新到最新版本）
#VERSION=${feat\/jsonschema:-latest}

# 红色加粗字体的
RED_BOLD="\033[1;31m"
# 重置终端颜色和样式
RESET="\033[0m"

# 用于查找所有包含 go.mod 的子模块
find_submodules() {
  find . -type f -name "go.mod" -exec dirname {} \;
}

# 更新指定依赖到最新版本
update_dependency() {
  local mod_path=$1
  echo ""
  echo -e "${RED_BOLD}Updating dependency ${DEPENDENCY}@feat/jsonschema in module $mod_path...${RESET}"
  echo ""

  # 切换到子模块目录
  cd "$mod_path" || exit

  # 更新依赖到最新版本
  go get -u "${DEPENDENCY}@feat/jsonschema"

  # 整理 go.mod 和 go.sum 文件
  go mod tidy

  # 切换回原始目录
  cd - >/dev/null || exit
}

# 遍历所有子模块并更新依赖
update_all_submodules() {
  submodules=$(find_submodules)

  if [ -z "$submodules" ]; then
    echo -e "${RED_BOLD}No Go modules found.${RESET}"
    exit 1
  fi

  for mod in $submodules; do
    update_dependency "$mod"
  done
}

# 开始更新
update_all_submodules

echo ""
echo -e "Dependency update completed."