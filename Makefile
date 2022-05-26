

.PHONY: build
build:
	go build -o ./dist/vmware_exporter vmware_exporter.go


.PHONY: release
release: build
	./tools/release


.PHONY: drone
drone:
	drone sign --save grafana/vmware_exporter
