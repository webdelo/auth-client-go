---
prefix: MKR
name: Mockery
packages:
  - github.com/vektra/mockery/v2
triggers:
  - mockery
  - generate mock
  - .mockery.yaml
  - мок
description: "CLI mock generator from Go interfaces; config via .mockery.yaml, generates mock structs with Expecter API"
---

# Mock Generation Using Mockery

## MKR-001: Mock Generation Workflow

- [ ] Analyze newly added interfaces or changes to existing interfaces.
- [ ] For new interfaces, add an entry to the `.mockery.yaml` config file in project root
- [ ] Run the [command](../../.memory_bank/dev_commands.yaml) to generate mocks

## MKR-002: Mockery Template

Standard settings for generating mocks in a package for testing. This example shows mock generation
for all interfaces present in the package.
```yaml
---
disable-version-string: true
resolve-type-alias: false
issue-845-fix: true
with-expecter: true
log-level: app_error

packages:
  gitlab.lamoda.tech/order/warden/app/services/deliverydates:
    config:
      all: true
      outpkg: mocks
      dir: "{{.InterfaceDir}}/mocks"
```

If you need to select specific interfaces, you can specify them in the `interfaces` list:
```yaml
---
disable-version-string: true
resolve-type-alias: false
issue-845-fix: true
with-expecter: true
log-level: app_error

packages:
  gitlab.lamoda.tech/order/warden/app/api/support:
    config:
      all: false
      outpkg: mocks
      dir: "{{.InterfaceDir}}/mocks"
    interfaces:
      monitoringPromisesSupportProcessor:
      allowedPalletMethodProcessor:
      delayMonitoringPromisesSupportProcessor:
```
**Target approach** - generate mocks for all interfaces present in the package.

## MKR-003: gRPC Server Mocks

For gRPC server interfaces, use the custom template `grpc_testify.tmpl` at `templates/mockery/`:

```yaml
packages:
  google.golang.org/grpc/interop/grpc_testing:
    config:
      include-auto-generated: true
      template: "file://templates/mockery/grpc_testify.tmpl"
      pkgname: grpc_mocks
      dir: "runner/mocks/grpc_testing"
      template-data:
        grpc-embed: true
    interfaces:
      TestServiceServer:
        config:
          structname: MockTestServiceServer
```

Key points:
- Set `template-data.grpc-embed: true` to embed `UnimplementedXxxServer` into the mock struct
- The template automatically derives the Unimplemented struct name from the interface name
- `include-auto-generated: true` is required because gRPC `.pb.go` files are auto-generated
- The embedded struct satisfies the `mustEmbedUnimplemented*()` constraint
- Explicit mock methods (`EmptyCall()`) override the embedded no-op implementations
