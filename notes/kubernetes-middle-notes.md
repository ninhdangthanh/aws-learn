# Kubernetes Middle Backend Notes

Note này gom các khái niệm Kubernetes ở mức **Middle Backend cần biết để trả lời phỏng vấn**, đúng theo góc nhìn của một backend developer chạy service trên Amazon EKS: hiểu khái niệm và dùng Helm để deploy, còn việc tạo/quản lý cluster do DevOps phụ trách.

Nguyên tắc trả lời: nói đúng phạm vi mình chạm tới. Không cần giả vờ tự dựng cluster — hiểu Cluster/Control Plane/Node/Pod/Deployment/Ingress và biết chỉnh `values.yaml` của Helm là đủ mạnh cho vai trò backend.

---

## 1. Kubernetes là gì?

> Kubernetes là một nền tảng **container orchestration** dùng để quản lý và vận hành các Docker container trên nhiều máy chủ. Nó giúp tự động deploy, scale, restart và cập nhật ứng dụng mà không cần thao tác thủ công.

**Nếu interviewer hỏi "Tại sao phải dùng Kubernetes?":**

> Nếu chỉ có một vài container thì Docker là đủ. Nhưng khi hệ thống có nhiều service chạy trên nhiều server, quản lý thủ công sẽ rất khó. Kubernetes tự động phân phối container lên các máy, tự restart khi container lỗi, hỗ trợ autoscaling và rolling update.

---

## 2. Cluster, Control Plane và Node

Đây là kiến trúc cơ bản nhất của Kubernetes.

```text
Kubernetes Cluster
├── Control Plane
└── Worker Nodes
      ├── Node 1
      ├── Node 2
      └── Node 3
```

**Cluster** — toàn bộ hệ thống Kubernetes. Ví dụ 1 Control Plane + 5 Worker Node tạo thành một Kubernetes Cluster.

**Control Plane** — bộ não của Kubernetes, chịu trách nhiệm:

* Nhận lệnh deploy.
* Quyết định Pod chạy ở Node nào (scheduling).
* Theo dõi trạng thái của cluster.
* Nếu Pod chết thì tạo Pod mới.

> Trong Amazon EKS, **AWS quản lý Control Plane** nên Backend Developer thường không phải cấu hình hay bảo trì phần này.

**Node** — một máy chạy workload. Trên AWS, một Node thường chính là một **EC2 instance**.

```text
Node A            Node B
- payment Pod     - order Pod
- user Pod        - report Pod
```

**Nếu hỏi "Pod chạy ở đâu?":** Pod luôn chạy trên một Worker Node.

---

## 3. Pod

Câu hỏi xuất hiện rất nhiều.

> Pod là **đơn vị triển khai (deploy) nhỏ nhất** trong Kubernetes.

Thông thường **1 Pod = 1 Docker Container**. Một Pod *có thể* chứa nhiều container, nhưng đa số ứng dụng backend là 1 Pod = 1 Container.

```text
payment-service → Deployment → 3 Pods → 3 Containers
```

Nếu một Pod crash, Kubernetes sẽ tạo Pod mới để đảm bảo số lượng Pod luôn đúng như mong muốn.

---

## 4. Deployment và Replica

Deployment dùng để **quản lý Pod**.

```yaml
replicas: 3
```

Kubernetes sẽ luôn cố duy trì 3 Pod. Nếu Pod 2 crash, Deployment tạo lại một Pod mới để vẫn đủ 3 Pod.

Deployment cũng hỗ trợ:

* Rolling Update.
* Rollback.
* Scale số lượng Pod.

**Nếu hỏi "Replica là gì?":**

> Replica là số lượng Pod mà mình mong muốn ứng dụng luôn duy trì. Ví dụ `replicas = 3` thì Kubernetes đảm bảo luôn có 3 Pod đang chạy.

---

## 5. Ingress

Ingress dùng để **định tuyến (route) request từ bên ngoài** vào các service trong cluster.

```text
Internet → Application Load Balancer → Ingress → service
```

* Request `/api/payment` → Ingress chuyển tới `payment-service`.
* Request `/api/user` → Ingress chuyển tới `user-service`.

> Trong AWS, Ingress thường được tích hợp với **Application Load Balancer (ALB)** để nhận traffic từ Internet rồi chuyển tiếp vào các service trong Kubernetes.

---

## 6. Helm

Trả lời phần này đúng với kinh nghiệm thực tế của mình.

> Helm là **package manager của Kubernetes**. Nó giúp quản lý và deploy các tài nguyên Kubernetes bằng template thay vì phải viết nhiều file YAML thủ công.

**"Bạn đã dùng Helm như thế nào?"** — câu trả lời đúng với CV:

> Em chủ yếu chỉnh sửa file `values.yaml`, thêm/sửa các environment variables.

```yaml
image:
  repository: payment-service
  tag: v1.2.0

replicaCount: 3

env:
  LOG_LEVEL: info
```

* Deploy version mới: đổi `tag: v1.3.0`.
* Scale: đổi `replicaCount: 5`.
* Sau đó chạy lệnh deploy của Helm (hoặc pipeline CI/CD thực hiện giúp).

---

## 7. Câu trả lời tổng hợp: "Bạn có kinh nghiệm với Kubernetes không?"

Phiên bản phù hợp với kinh nghiệm thực tế (backend chạy trên EKS, DevOps quản cluster):

> "Ở công ty em sử dụng **Amazon EKS** để chạy các backend services. Em không trực tiếp tạo hay quản lý cluster vì phần đó do DevOps phụ trách. Tuy nhiên, em hiểu các khái niệm cơ bản của Kubernetes như **Cluster, Control Plane, Worker Node, Pod và Deployment**. Em biết **Deployment** quản lý các Pod và đảm bảo số lượng replica luôn đúng, đồng thời hỗ trợ **rolling update** khi deploy phiên bản mới. Em cũng hiểu **Ingress** dùng để định tuyến request từ ALB vào các service trong cluster. Trong công việc hằng ngày, em chủ yếu dùng **Helm** để cập nhật `values.yaml` và các cấu hình môi trường trước khi deploy ứng dụng lên EKS."

---

## 8. ECS vs Kubernetes (EKS)

Câu hỏi hay gặp khi đã nói mình chạy trên AWS. Ý chính: **ECS đơn giản và gắn chặt AWS; Kubernetes mạnh, linh hoạt, đa cloud nhưng phức tạp hơn.**

> **ECS** là dịch vụ container orchestration do AWS phát triển, dễ sử dụng và tích hợp rất tốt với các dịch vụ AWS. Nó phù hợp nếu hệ thống chỉ chạy trên AWS và không cần nhiều cấu hình phức tạp.
>
> **Kubernetes** (trên AWS thường dùng qua **EKS**) là nền tảng orchestration tiêu chuẩn của ngành, mạnh và linh hoạt hơn với nhiều tính năng như scheduling nâng cao, autoscaling, service discovery và khả năng chạy đa cloud. Đổi lại, Kubernetes có độ phức tạp và chi phí vận hành cao hơn ECS.
>
> Một điểm quan trọng nữa: **ECS dính chặt với AWS (vendor lock-in)**, còn **Kubernetes thì không** — có thể di dời qua các cloud khác như Azure hay GCP dễ dàng.

| Tiêu chí | ECS | Kubernetes (EKS) |
|---|---|---|
| Nhà phát triển | AWS | CNCF (chuẩn ngành) |
| Độ phức tạp | Thấp, dễ dùng | Cao hơn, nhiều khái niệm |
| Tích hợp AWS | Rất sâu, gọn | Tốt nhưng cần cấu hình thêm |
| Tính năng | Đủ dùng | Mạnh hơn: scheduling nâng cao, autoscaling, service discovery |
| Đa cloud (portability) | Lock-in vào AWS | Chạy được AWS/Azure/GCP/on-prem |
| Chi phí vận hành | Thấp | Cao hơn |

**Khi nào chọn cái nào:** chỉ chạy trên AWS, team nhỏ, muốn đơn giản → ECS. Cần linh hoạt cao, đa cloud, hệ sinh thái Kubernetes (Helm, operator...), tránh vendor lock-in → EKS.

---

## Liên kết

* [Backend Classic Questions](backend-classic-questions.md) — câu hỏi tình huống production
* [Scale System Questions](scale_system_question.md) — các tầng của một request và scale path
