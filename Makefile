ALL_GO_MOD_DIRS := $(shell find . -type f -name 'go.mod' -exec dirname {} \; | sort)

GO = go

# ===================================
# Go Modules

.PHONY: go-mod-tidy
# Usage:
# make go-mod-tidy         # Tidy all subprojects
# make go-mod-tidy/app/dir # Tidy a specific subproject
go-mod-tidy: $(ALL_GO_MOD_DIRS:%=go-mod-tidy/%)
go-mod-tidy/%: DIR=$*
go-mod-tidy/%:
	@echo "$(GO) mod tidy in $(DIR)" \
		&& cd $(DIR) \
		&& $(GO) mod tidy -compat=1.22.0

.PHONY: go-mod-update
# Usage:
# make go-mod-update                                           # Update all packages in all subprojects (use with caution)
# make go-mod-update PACKAGE=github.com/cloudwego/eino         # Update specified package in all subprojects
# make go-mod-update PACKAGE=github.com/cloudwego/eino/...     # Update specified package and its subpackages in all subprojects
# make go-mod-update/app/dir                                   # Update all packages in a specific subproject
# make go-mod-update/app/dir PACKAGE=github.com/cloudwego/eino # Update specified package in a specific subproject
go-mod-update: $(ALL_GO_MOD_DIRS:%=go-mod-update/%)
go-mod-update/%: DIR=$*
go-mod-update/%:
	@echo "$(GO) mod update in $(DIR)" \
		&& cd $(DIR) \
		&& if [ -z "$(PACKAGE)" ] || grep -q "$(PACKAGE)" go.mod; then \
		  	echo "😄update: $(DIR) need package $(PACKAGE)"; \
			$(GO) get -u $(if $(PACKAGE),$(PACKAGE),./...); \
		else \
		  	echo "😐skip: $(DIR) does not need package $(PACKAGE)"; \
		fi

.PHONY: go-mod-list
# Usage:
# make go-mod-list         # List all dependencies in all subprojects
# make go-mod-list/app/dir # List dependencies in a specific subproject
go-mod-list: $(ALL_GO_MOD_DIRS:%=go-mod-list/%)
go-mod-list/%: DIR=$*
go-mod-list/%:
	@echo "$(GO) list -m all in $(DIR)" \
		&& cd $(DIR) \
		&& $(GO) list -m all