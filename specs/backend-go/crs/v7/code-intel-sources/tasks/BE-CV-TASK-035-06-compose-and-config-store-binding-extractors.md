# BE-CV-TASK-035-06: Trích Store và Binding từ compose, init script, `config.go` và thư mục adapter

**From Solution:** BE-CV-SOL-035-storage-map
**Priority:** P2
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/composesource/compose_document.go`, `init_databases_script.go`; `internal/adapter/gosourcescan/service_config_env_keys.go`, `adapter_directory_kinds.go`; các `_test.go`; `internal/usecase/ports.go` (sửa: thêm `ComposeDocumentParser`, `GoSourceScanner`); `go.mod` của `code-intel-service` (thêm `gopkg.in/yaml.v3 v3.0.1`) (mới/sửa)
**Depends on:** BE-CV-TASK-035-03, BE-CV-TASK-035-04
**Status:** [ ] TODO

---

## Context

Nguồn của Store/Binding. Mọi hàm nhận **`[]byte`/danh sách tên thư mục** (cổng đọc file đã lấy nội dung ở TASK-035-08), nên test chạy offline trên fixture. `yaml.v3 v3.0.1` đã là dependency trực tiếp của `api-gateway` và `infra-fleet-service` nên dùng đúng phiên bản; thêm vào module mới vẫn là thay đổi `go.mod` cần review. Build `CGO_ENABLED=0` không bị ảnh hưởng.

## Việc cần làm

1. `compose_document.go`: `ParseCompose(content []byte, env Environment, file string) (ComposeFacts, error)`. Dùng `yaml.v3` vào `yaml.Node`, **tự giải anchor và merge key** (`<<: *go-defaults`, `<<: *go-common-env`) vì giá trị `environment` kế thừa. Chỉ đọc trường allowlist (solution 035 §2.E.2); `command`, `healthcheck`, `logging`, `labels`, comment: không bao giờ chạm. `environment` ở dạng map **và** dạng list `KEY=VALUE` đều phải hỗ trợ. Mọi giá trị `environment` đi qua `secretmasking.Decide` (TASK-035-04). Trả `ComposeFacts{Project string, Services []ComposeService{Name, ContainerName, Image, Profiles, Volumes []VolumeMount, EnvKeys []EnvFact}, TopLevelVolumes []string}`.
2. Store từ compose: theo bảng solution 035 §2.C: `postgres`, `nats` (kind `queue`), volume top-level, Store `vault` shared-external suy từ `VAULT_ADDR` trong `x-go-common-env` (chỉ khi key có mặt; `external=true`, `deployed=false`), Store `mysql` `supportedByCode` (do TASK-035-06 mục 5 sau khi biết adapter). `Store.name` = `container_name` nếu có, không thì `<name dự án>-<service>`; compose prod: chỉ khối `orca-server` + volume `orca-data`, thêm cảnh báo `prod_topology_unknown`.
3. `init_databases_script.go`: `ParseDatabaseNames(content []byte) ([]string, error)`: lấy giá trị biến `DATABASES="…"` bằng tách dòng và kiểm văn phạm `^[a-z0-9_]{1,63}$` từng tên; **không** thực thi script, không đoán biến khác; thiếu biến ⇒ lỗi (không rỗng im lặng). Mong đợi 17 tên trên repo thật.
4. `service_config_env_keys.go`: `ScanEnvKeys(content []byte, file string) ([]EnvKey, error)` dùng `go/parser` (+ `ast.Inspect`): gọi `os.Getenv("X")`, `os.LookupEnv("X")`, `commonconfig.StringEnv("X", …)` (khớp theo tên selector cuối `StringEnv`), và hàm cục bộ có tên khớp `^[a-z][A-Za-z]*Env$` với đối số đầu là literal chuỗi (`intEnv`, `boolEnv`). Chỉ trả **tên khoá** (`EnvKey{Name, Line, Class}`); `Class` theo bảng: `*_ADDR`⇒`dependency`, `NATS_URL`⇒`nats`, `DATABASE_*`⇒`database`, `VAULT_*`/`*_CREDENTIALS_FILE`⇒`vault`, còn lại `other`. Giá trị mặc định literal chỉ đưa vào `Default` khi khoá nằm allowlist và qua `Decide`.
5. `adapter_directory_kinds.go`: `ClassifyAdapterDirs(dirNames []string) []AdapterKind` theo bảng cố định (`postgres`, `mysql`, `eventbus|natsconsumer`⇒nats, `vault|vaultsigner`⇒vault, `localfs|localgit`⇒volume, `cache`⇒bỏ, mọi tên khác bỏ). Tạo `Binding{service, store, access:"rw", via:<tên thư mục>, config_key, confidence:"derived"}`; `config_key` = tên khoá lớp tương ứng (`DATABASE_DSN` cho `postgres|mysql`, `NATS_URL` cho nats, `DATABASE_CREDENTIALS_FILE`/`VAULT_*` cho vault). `Binding.database` = `DBName` của `DATABASE_DSN` ở khối compose cùng tên service (bỏ trống nếu không có, ví dụ MySQL không compose); `confidence` nâng thành `declared` khi cả hai nguồn khớp.
6. Service có `adapter/mysql` mà không compose nào có MySQL ⇒ thêm Store `mysql:dev:supported-by-code` (`deployed=false`) và Binding `confidence:"derived"`; `mcp-service` không có `adapter/mysql` ⇒ không có Binding MySQL. `git-gateway-service`/`api-gateway` không có adapter DB ⇒ không có Binding DB.

## Kiểm thử

- `go test ./services/code-intel-service/internal/adapter/composesource/... ./internal/adapter/gosourcescan/...` trên fixture `testdata/storage` (TASK-035-01): anchor/merge key; `environment` dạng list; `command` không xuất hiện trong kết quả; DSN với `${POSTGRES_PASSWORD}`; ngoại lệ `VAULT_TOKEN: ${VAULT_TOKEN:?…}` ⇒ `NameOnly`; `init-databases.sh` ⇒ 17 tên; `StringEnv` và `intEnv`; service không theo mẫu config ⇒ trả danh sách rỗng **kèm** `warnings`, không panic; adapter dirs của 4 service mẫu.
- Test bảo mật: quét byte `ComposeFacts` serialize không chứa `CANARY_`.
- Test chống sai: compose YAML hỏng ⇒ `error`, không panic; YAML có `!!binary`/alias bomb nhỏ ⇒ giới hạn độ sâu/kích thước (≤ 512 KiB tệp, ≤ 10 000 node), vượt ⇒ lỗi.

## Tiêu chí hoàn thành

- [ ] Bảng Store/Binding mong đợi (solution 035 §9) đạt trên fixture.
- [ ] `command`, comment, giá trị khoá nhạy cảm không xuất hiện ở bất kỳ đầu ra nào.
- [ ] `yaml.v3` là dependency duy nhất thêm; `go mod tidy` không kéo thứ khác (chưa chạy).

## Rủi ro và lưu ý

- Giải merge key bằng `yaml.Node` dễ sai thứ tự ghi đè (khoá cục bộ thắng `<<`); có test riêng cho trường hợp ghi đè.
- Chưa kiểm chứng mọi service dùng cùng mẫu env (TASK-035-01 mục 3 sẽ liệt kê ngoại lệ).
- `localfs|localgit` là quy tắc thêm (C6); nếu không có volume mount cùng tên trong compose thì Binding bị bỏ chứ không bịa Store.
