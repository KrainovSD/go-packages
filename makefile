ifeq ($(OS),Windows_NT)
RM := del /Q
DOC_FILE := internal\docs\docs.go
AIR := air -c .air_windows.toml
else
RM := rm -f
DOC_FILE := ./internal/docs/docs.go
AIR := air -c .air.toml
endif

dev:
	swag init -g ./internal/main.go -o ./internal/docs 
	$(RM) $(DOC_FILE) 
	$(AIR)
