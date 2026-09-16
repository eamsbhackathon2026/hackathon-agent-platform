# Agent Platform

Nền tảng tạo và vận hành trợ lý AI trong một workspace chung. Backend viết bằng Go
theo kiến trúc hexagonal, trang quản trị viết bằng React theo Feature-Sliced Design,
hợp đồng API dùng chung mô tả bằng OpenAPI tại `api/openapi.yaml`.

Kho mã này là bản phát hành: chỉ chứa mã nguồn chạy được, cấu hình triển khai và
script vận hành. Mỗi phiên bản là một commit kèm tag.

## Cấu trúc thư mục

```text
.
├── api/openapi.yaml           Hợp đồng API, nguồn sinh mã cho backend và frontend
├── services/agent-service     Backend Go
│   ├── cmd
│   │   ├── agent-service      Tiến trình API kèm worker
│   │   ├── dev-env            Nạp biến môi trường cho lệnh phát triển
│   │   └── migrate            Chạy migration khi triển khai
│   ├── internal
│   │   ├── core               Nghiệp vụ thuần: domain, ports, services
│   │   ├── adapters
│   │   │   ├── inbound        HTTP handler và worker tiêu thụ hàng đợi
│   │   │   └── outbound       Postgres, nhà cung cấp AI, công cụ, webhook, mã hóa
│   │   ├── platform           Tiện ích hạ tầng: clock, logger, kiểm soát egress
│   │   ├── config             Đọc và kiểm tra cấu hình khởi động
│   │   └── testsupport        Tiện ích dùng chung cho kiểm thử
│   ├── migrations             Migration Postgres theo thứ tự
│   ├── oapi-codegen.yaml      Cấu hình sinh mã Go từ OpenAPI
│   └── sqlc.yaml              Cấu hình sinh truy vấn từ migration
├── apps/admin-web             Trang quản trị React
│   └── src
│       ├── app                Khởi tạo ứng dụng, router, provider, style
│       ├── pages              Màn hình theo route
│       ├── widgets            Khối giao diện ghép từ nhiều feature
│       ├── features           Hành vi người dùng có thao tác ghi
│       ├── entities           Mô hình nghiệp vụ và truy vấn dữ liệu
│       └── shared             Thư viện UI, client API, tiện ích dùng chung
├── deploy
│   ├── docker                 Dockerfile backend, frontend và cấu hình Nginx
│   ├── k8s                    Manifest Kubernetes theo thứ tự áp dụng
│   └── *.sh                   Script build image, tạo secret và áp dụng manifest
├── scripts                    Tiện ích sinh mã và tạo migration
├── Makefile                   Lệnh phát triển, kiểm tra và build
├── docker-compose.yml         Postgres cho môi trường phát triển
└── .env.example               Danh sách biến môi trường kèm giá trị mẫu
```

Backend theo kiến trúc hexagonal: `core` không phụ thuộc hạ tầng, mọi truy cập ra
ngoài đi qua cổng khai báo trong `core/ports` và được hiện thực trong `adapters`.
Nhờ vậy thay nhà cung cấp AI hay đổi kho dữ liệu không làm thay đổi nghiệp vụ.

Frontend theo Feature-Sliced Design, các lớp chỉ phụ thuộc theo một chiều từ trên
xuống trong danh sách trên: `app` dùng được mọi lớp dưới, còn `shared` không phụ
thuộc lớp nào.

## Yêu cầu môi trường

Go có khả năng tải toolchain tự động, Node.js 22.19, pnpm 11.20 và Docker Compose.
Hỗ trợ macOS và Linux.

## Cấu hình

```sh
cp .env.example .env
```

Sinh ba giá trị độc lập cho `APP_ENCRYPTION_KEY`, `JWT_SIGNING_KEY` và
`API_KEY_PEPPER`. Mỗi giá trị là base64 của đúng 32 byte, chạy lệnh sau riêng cho
từng khóa:

```sh
openssl rand -base64 32
```

Điền `CORS_ORIGINS` bằng các origin của trang quản trị được phép, khớp chính xác
scheme, host và port. `JWT_ISSUER` mặc định `agent-platform`, `JWT_AUDIENCE` mặc
định `agent-platform-admin`.

Kết nối AI nội bộ ngoài môi trường dev cần HTTPS và `EGRESS_ALLOWLIST` chứa
hostname chính xác hoặc dải CIDR, ví dụ `ai.internal.example,10.20.0.0/16`. Địa chỉ
IP đơn dùng `/32` hoặc `/128`, không dùng URL hay ký tự đại diện. Endpoint metadata
của nhà cung cấp hạ tầng luôn bị chặn.

`WORKER_ENABLED=true` bật worker xử lý run, webhook và bảo trì trong cùng tiến
trình. `WORKER_CONCURRENCY` mặc định 4 và nhận giá trị từ 1 đến 64. Khi tắt worker,
yêu cầu chạy bất đồng bộ vẫn được xếp hàng cho instance khác xử lý.

Mọi giá trị `change-me` trong `.env.example` chỉ là mẫu. Đổi mật khẩu Postgres
đồng thời tại `POSTGRES_PASSWORD` và `DATABASE_URL`. Mật khẩu chứa `$` cần đặt
trong nháy đơn, và trong `DATABASE_URL` phải mã hóa theo URL, ví dụ `$` thành `%24`.

## Chạy

```sh
pnpm install --frozen-lockfile
make up
make gen
make migrate-up
make dev-api
# ở terminal khác
make dev-web
```

API mặc định tại `http://127.0.0.1:8080`, trang quản trị tại
`http://localhost:5173` và proxy `/v1` sang backend để cookie hoạt động cùng
origin. Kiểm tra `GET /healthz` cho tiến trình và `GET /readyz` cho kết nối
Postgres. Lần đầu khởi động, `GET /v1/auth/config` cho biết có thể thiết lập tài
khoản owner đầu tiên hay không.

`make down` dừng Postgres và giữ nguyên volume dữ liệu. `make logs` xem log Postgres.

## Kiểm tra chất lượng

```sh
make tools
make lint test build
make check-gen
make test-integration
```

`make tools` cài golangci-lint đúng phiên bản được ghim vào `bin/`. `make check-gen`
xác nhận mã sinh từ OpenAPI còn khớp hợp đồng. `make check-contract` xác nhận mọi
tag trong OpenAPI đã được bật ở server. Kiểm thử tích hợp cần Docker, khởi động
Postgres 18 bằng testcontainers, hoặc dùng schema tách biệt nếu có `TEST_DATABASE_URL`.

## Triển khai

`deploy/docker` chứa Dockerfile cho backend và trang quản trị. `deploy/k8s` chứa
manifest theo thứ tự namespace, Postgres, job migration, backend, frontend và
ingress. `deploy/publish-images.sh` build và đẩy image, `deploy/create-secrets.sh`
tạo secret, `deploy/apply.sh` áp dụng manifest.

Chạy job migration trước khi đưa phiên bản mới vào phục vụ. Không đặt giá trị bí
mật thật trong lệnh terminal, trong tài liệu hay trong manifest được commit.

### CI/CD

`.github/workflows/agent-platform.yml` chạy khi push vào `main` (trừ tệp `.md`),
hoặc bấm tay qua *Run workflow*. Ba job nối tiếp:

| Job | Nội dung |
|---|---|
| `test` | `make test-go` (`go test -race`), `make test-web` (Vitest), `make build` — `build-web` chạy `tsc -b` nên lỗi kiểu bị chặn tại đây |
| `build-and-push` | Build hai image cho `linux/amd64`, đẩy vCR với tag commit SHA và `latest` |
| `deploy` | Tạo namespace, tạo `vcr-cred`, kiểm tra Secret ứng dụng, rồi gọi `deploy/apply.sh` |

Job `deploy` không tự viết lại thứ tự triển khai mà gọi thẳng `deploy/apply.sh`,
để chỉ có một nguồn quyết định thứ tự: chờ Postgres sẵn sàng → xoá Job migration
cũ (Job là bất biến) → chạy bản mới và đợi xong → mới đưa image mới vào phục vụ.

Ba secret cần có trong repo GitHub: `VCR_USERNAME`, `VCR_PASSWORD`, `KUBE_CONFIG`
(kubeconfig đã base64). Không giá trị nào được commit.

**CI không tạo được `postgres-credentials` và `agent-service-config`.** Hai Secret
này chứa mật khẩu Postgres và ba khóa 32 byte, và phải giữ nguyên giữa các lần
rollout — đổi khóa là mất hiệu lực toàn bộ token cùng dữ liệu đã mã hóa. Chúng
được sinh một lần bằng `deploy/create-secrets.sh` ở máy và lưu tại
`deploy/.env.production`. Job `deploy` chỉ kiểm tra sự tồn tại rồi dừng kèm hướng
dẫn nếu thiếu.

Trình tự lần đầu:

```sh
export KUBECONFIG=...                 # trỏ đúng cluster VKS
sh deploy/create-secrets.sh           # lần 1: sinh file, báo thiếu APP_ORIGIN
# điền APP_ORIGIN trong deploy/.env.production, xem mục dưới
sh deploy/create-secrets.sh           # lần 2: tạo Secret trên cluster
git push                              # CD lo phần còn lại
```

**`APPLY_INGRESS` mặc định `0` trong workflow.** `deploy/k8s/50-ingress.yaml` tạo
ALB internet-facing trên vLB — tài nguyên tính phí và đưa trang quản trị ra
Internet. Bản triển khai hiện tại vào bằng `kubectl port-forward`:

```sh
kubectl -n agent-platform port-forward svc/admin-web 8090:8080
kubectl -n agent-platform port-forward svc/agent-service 8091:8080
```

Vì vậy `APP_ORIGIN` phải là `http://localhost:8090` — đúng cổng bạn port-forward.
`create-secrets.sh` đưa giá trị này vào `CORS_ORIGINS`; đặt sai thì trang tải được
nhưng mọi POST/PUT/DELETE bị chặn 403 mà giao diện không nói rõ lý do.

Muốn có địa chỉ công khai thì đổi `APPLY_INGRESS` thành `"1"` trong workflow, chạy
lại, lấy IP từ `kubectl -n agent-platform get ingress agent-platform`, cập nhật
`APP_ORIGIN` thành IP đó rồi chạy lại `create-secrets.sh` và khởi động lại
`agent-service`. Chạy `deploy/apply.sh` bằng tay vẫn tạo ingress như trước, trừ khi
đặt `APPLY_INGRESS=0`.

### imagePullSecrets

Ba manifest kéo image (`20-migrate-job`, `30-agent-service`, `40-admin-web`) khai
báo `imagePullSecrets: vcr-cred`. Project trên vCR là private — request tới
registry không kèm credential trả về `401` — nên thiếu khai báo này thì pod kẹt ở
`ImagePullBackOff` và `apply.sh` treo tới hết timeout 5 phút. Secret `vcr-cred`
do job `deploy` tạo từ `VCR_USERNAME`/`VCR_PASSWORD`.
