# CRM SaaS --- Product Brainstorming & Architecture Direction

## 1. Product Vision

Produk yang ingin dibangun adalah **Multi-Tenant CRM SaaS** yang tidak
hanya menyediakan dashboard CRM, tetapi juga menyediakan **Lead Capture
& Integration Platform**.

Core value:

> Capture leads from anywhere → manage leads → assign employees → follow
> up → convert to customers.

Sumber lead dapat berasal dari:

-   Direct API / HTTP POST
-   Embedded Form
-   Inbound Webhook
-   Integrasi pihak ketiga

Lead yang masuk akan diproses di CRM, kemudian dapat di-assign ke
employee yang menggunakan aplikasi mobile Flutter.

------------------------------------------------------------------------

# 2. High-Level Product Flow

``` text
                    ┌──────────────────────┐
                    │    LANDING PAGE      │
                    │      crm.com         │
                    └──────────┬───────────┘
                               │
                    Marketing / Pricing
                               │
                         [ Get Started ]
                               │
                               ▼
                    ┌──────────────────────┐
                    │      REGISTER        │
                    │                      │
                    │ Organization Name    │
                    │ Owner Name           │
                    │ Email                │
                    │ Password             │
                    └──────────┬───────────┘
                               │
                         Verify Email
                               │
                               ▼
                    ┌──────────────────────┐
                    │        LOGIN         │
                    └──────────┬───────────┘
                               │
                               ▼
        ╔══════════════════════════════════════════════╗
        ║              CRM DASHBOARD                  ║
        ║          Multi-Tenant Application            ║
        ╚══════════════════════════════════════════════╝
                               │
             ┌─────────────────┼─────────────────┐
             │                 │                 │
             ▼                 ▼                 ▼
        CRM / Leads        Employees        Subscription
             │                 │                 │
             │                 │                 ▼
             │                 │              Billing
             │                 │                 │
             │                 │              API Keys
             │                 │
             │                 ▼
             │             Flutter App
             │             Employee
             │
             ▼
       Lead Capture
             │
     ┌───────┼────────┐
     │       │        │
     ▼       ▼        ▼
   Embed    API    Webhook
    Form    POST
     │       │        │
     └───────┼────────┘
             ▼
           LEAD
             │
             ▼
       CRM Dashboard
             │
             ▼
        Assignment
             │
             ▼
         Employee
             │
             ▼
       Follow Up
             │
             ▼
        Customer
```

------------------------------------------------------------------------

# 3. Product Areas

Produk secara umum dibagi menjadi 4 bagian.

## 3.1 Marketing / Landing Page

Technology: **Next.js**

Contoh:

``` text
crm.com
```

Landing page hanya berfungsi sebagai marketing dan entry point.

Fitur:

-   Hero / product positioning
-   Product features
-   Use cases
-   Pricing
-   Documentation
-   Register
-   Login
-   Contact

Landing page **tidak menangani operational CRM**.

Prinsip:

> Landing page = menjual produk.
>
> Dashboard = menggunakan produk.

------------------------------------------------------------------------

## 3.2 Owner CRM Dashboard

Technology: **Next.js**

Contoh:

``` text
app.crm.com
```

Ini adalah aplikasi CRM utama dan bersifat **multi-tenant**.

Owner/Admin/Manager dapat melakukan:

-   Dashboard
-   Lead management
-   Customer management
-   Pipeline
-   Task
-   Employee management
-   Form management
-   Integration management
-   API Key management
-   Subscription
-   Billing
-   Settings
-   Reports
-   Audit Log

------------------------------------------------------------------------

## 3.3 Employee Mobile App

Technology: **Flutter**

Employee tidak perlu menggunakan dashboard owner.

Mobile app fokus pada pekerjaan operasional employee:

-   Login
-   My Tasks
-   My Leads
-   Assigned Leads
-   Customer
-   Follow Up
-   Activity
-   Notifications
-   Call
-   WhatsApp
-   Update Lead Status
-   Add Notes
-   Profile

Contoh workflow:

``` text
New Lead Assigned
       ↓
Employee receives notification
       ↓
Open Lead
       ↓
Call / WhatsApp
       ↓
Add Activity
       ↓
Schedule Follow Up
       ↓
Update Status
```

------------------------------------------------------------------------

## 3.4 Backend / API

Technology:

-   Go
-   Gin
-   PostgreSQL
-   Docker / Docker Compose

Contoh:

``` text
api.crm.com
```

Backend menjadi pusat seluruh sistem.

------------------------------------------------------------------------

# 4. Registration & Onboarding Flow

Landing page tidak dibuat terlalu berat.

Flow:

``` text
Landing
   ↓
Pricing / Get Started
   ↓
Register
   ↓
Create Organization
   ↓
Verify Email
   ↓
Login
   ↓
Owner Dashboard
```

Saat register, user mengisi:

``` text
Organization Name
Owner Name
Email
Password
```

Backend membuat:

``` text
User
Organization
Membership
Subscription
```

Contoh:

``` text
User
 └── Budi Santoso

Organization
 └── PT ABC Indonesia

Membership
 └── Owner

Subscription
 └── Free
```

API Key **tidak harus otomatis dibuat saat registration**.

Lebih baik owner membuat API Key secara explicit dari dashboard.

------------------------------------------------------------------------

# 5. Email Verification

Setelah registration:

``` text
Create Account
      ↓
Verification Email
      ↓
User clicks verification link
      ↓
Email Verified
      ↓
Redirect to Login
```

Setelah login user masuk ke owner dashboard.

------------------------------------------------------------------------

# 6. Free Tier

Organization baru mendapatkan Free Tier.

Tujuannya agar user dapat mencoba produk sebelum membayar.

Contoh konsep Free Tier:

``` text
FREE

100 Leads / month
2 Employees
1 Form
1 API Key
Basic Dashboard
```

Angka limit di atas hanya contoh dan perlu ditentukan kembali
berdasarkan business model.

Jika user membutuhkan fitur/limit lebih tinggi:

``` text
Free
 ↓
Starter
 ↓
Business
```

------------------------------------------------------------------------

# 7. Subscription & Billing

Landing page hanya menampilkan pricing.

Contoh:

``` text
Pricing

Free
Starter
Business
```

Setelah login, owner dapat melihat:

``` text
Settings
   ↓
Subscription
```

Di dashboard owner dapat:

-   Melihat current plan
-   Melihat usage
-   Upgrade
-   Downgrade
-   Manage Billing
-   Melakukan pembayaran
-   Melihat billing status

Subscription menentukan capability/limit organization.

Jangan hanya membatasi jumlah user; subscription dapat menentukan:

-   Number of employees
-   Leads/month
-   Number of forms
-   API usage
-   Number of API keys
-   Webhook availability
-   Advanced reports
-   Automation
-   Other premium features

------------------------------------------------------------------------

# 8. API Key

API Key digunakan untuk authentication ketika sistem eksternal ingin
mengirim data ke CRM.

API Key dibuat dari:

``` text
Dashboard
   ↓
Settings
   ↓
Developer
   ↓
API Keys
   ↓
Create API Key
```

Contoh:

``` text
crm_live_xxxxxxxxx
```

API key sebaiknya:

-   Tidak dikirim melalui email
-   Hanya ditampilkan sekali setelah dibuat
-   Disimpan sebagai hash di database
-   Bisa di-revoke
-   Bisa di-disable
-   Bisa di-regenerate
-   Memiliki created_at
-   Memiliki last_used_at
-   Memiliki environment/name

------------------------------------------------------------------------

# 9. Kenapa Multiple API Keys Bisa Berguna?

User tidak wajib memiliki banyak API key.

Multiple API keys berguna jika organization memiliki beberapa
integration/environment.

Contoh:

``` text
Production Website
Development Website
Landing Page
Internal System
```

Contoh:

``` text
API Keys

Production Website
crm_live_xxxx

Development
crm_test_xxxx
```

Namun untuk MVP, jumlah API key tidak harus langsung kompleks.

Yang penting adalah fondasi API Key management sudah benar.

------------------------------------------------------------------------

# 10. API Key vs Form ID

API Key dan Form ID memiliki fungsi berbeda.

## API Key

Untuk authentication.

``` text
Website
   ↓
API Key
   ↓
CRM API
```

## Form ID

Untuk mengidentifikasi form tertentu.

Contoh:

``` text
form_01JABC
```

Misalnya organization memiliki:

``` text
Contact Us Form
Product Inquiry Form
Demo Request Form
```

Masing-masing memiliki Form ID.

Jadi request dapat secara konsep menggunakan:

``` http
POST /v1/forms/{form_id}/submit
Authorization: Bearer {api_key}
```

Backend dapat menentukan:

``` text
API Key
   ↓
Organization

Form ID
   ↓
Specific Form

Submission
   ↓
Create Lead
```

------------------------------------------------------------------------

# 11. Lead Capture Methods

CRM menyediakan beberapa cara untuk memasukkan lead.

## 11.1 Direct API / HTTP POST

Untuk customer yang memiliki website/backend sendiri.

Flow:

``` text
Customer Website
       ↓
POST /v1/leads
       ↓
Authorization: API Key
       ↓
CRM
       ↓
Lead
```

Contoh body:

``` json
{
  "name": "Budi Santoso",
  "email": "budi@gmail.com",
  "phone": "08123456789",
  "message": "Saya tertarik dengan produk"
}
```

Backend akan membuat Lead.

------------------------------------------------------------------------

# 12. Embedded Form

Customer yang tidak ingin membuat backend sendiri dapat menggunakan form
yang disediakan CRM.

Owner:

``` text
Dashboard
   ↓
Forms
   ↓
Create Form
```

Owner memilih fields:

``` text
Name
Email
Phone
Message
Company
Product
```

CRM menghasilkan embed/script code.

Contoh konsep:

``` html
<script src="https://cdn.crm.com/form.js"></script>
```

atau iframe/embed mechanism.

Customer cukup memasang code tersebut ke website mereka.

Flow:

``` text
Customer Website
       ↓
Embedded CRM Form
       ↓
Visitor submits form
       ↓
CRM
       ↓
Lead
```

------------------------------------------------------------------------

# 13. Webhook

Webhook digunakan untuk integrasi dengan sistem lain.

## Inbound Webhook

Third party → CRM.

Contoh:

``` text
WordPress
Shopify
Website
Other System
       ↓
Webhook
       ↓
CRM
       ↓
Lead
```

## Outbound Webhook

CRM → third party.

Contoh event:

``` text
lead.created
lead.updated
lead.assigned
lead.converted
task.created
```

Contoh konsep payload:

``` json
{
  "event": "lead.created",
  "data": {
    "id": "lead_123",
    "name": "Budi"
  }
}
```

Outbound webhook dapat digunakan agar sistem customer mengetahui event
yang terjadi di CRM.

------------------------------------------------------------------------

# 14. Semua Lead Capture Masuk ke Pipeline yang Sama

Tidak peduli sumbernya:

``` text
API
Embedded Form
Inbound Webhook
```

semuanya pada akhirnya menjadi:

``` text
LEAD
```

Contoh:

``` text
Lead #10291

Name:
Budi Santoso

Phone:
08123456789

Source:
Website Form

Status:
NEW
```

Ini adalah prinsip penting:

> Integration layer hanya menjadi cara memasukkan data. Core CRM tetap
> menggunakan model Lead yang sama.

------------------------------------------------------------------------

# 15. Assignment

Setelah Lead masuk, lead dapat di-assign ke employee.

Contoh manual:

``` text
Lead Budi
   ↓
Assign
   ↓
Andi
```

Database concept:

``` text
lead.assigned_to = employee_id
```

Nanti dapat dikembangkan menjadi Assignment Engine.

Contoh rule:

``` text
IF product = Furniture
→ Furniture Sales Team

IF source = Website
→ Sales Team

IF region = Jakarta
→ Employee A
```

Untuk MVP cukup:

-   Manual assignment
-   Round robin sederhana

------------------------------------------------------------------------

# 16. Employee Workflow

Employee menggunakan Flutter Mobile.

Flow:

``` text
Lead Created
      ↓
Assignment
      ↓
Employee
      ↓
Push Notification
      ↓
Flutter App
      ↓
Employee opens lead
      ↓
Follow Up
      ↓
Activity
      ↓
Task
      ↓
Update status
```

Contoh:

``` text
New Lead Assigned

Budi Santoso
Interested in Office Furniture

[Call]
[WhatsApp]
[Add Note]
[Follow Up]
[Change Status]
```

------------------------------------------------------------------------

# 17. Activity vs Task

Jangan menjadikan semua action sebagai Task.

Lead dapat memiliki Activity dan Task.

Contoh:

``` text
Lead Budi
   │
   ├── Activity: Lead Created
   ├── Task: Call customer
   ├── Activity: Customer called
   ├── Task: Send quotation
   ├── Activity: WhatsApp sent
   └── Task: Follow up tomorrow
```

Konsep:

-   Activity = sesuatu yang sudah terjadi
-   Task = sesuatu yang harus dilakukan

------------------------------------------------------------------------

# 18. Role & Permission

Karena sistem multi-tenant, role dan permission perlu dipikirkan sejak
awal.

Minimal role:

``` text
Owner
Admin
Manager
Employee
```

Contoh:

## Owner

-   Full CRM access
-   Manage organization
-   Manage subscription
-   Manage billing
-   Manage employees
-   Manage integrations

## Admin

-   Manage employees
-   Manage leads
-   Manage customers
-   Manage CRM operations

## Manager

-   View team
-   Assign leads
-   Manage tasks
-   Monitor team

## Employee

-   View assigned leads
-   Update tasks
-   Add activities
-   Follow up customer

Permission harus tetap dibatasi berdasarkan organization/tenant.

------------------------------------------------------------------------

# 19. Multi-Tenant Architecture

Backend sejak awal menggunakan multi-tenant architecture.

Core relationship:

``` text
Organization
    │
    ├── Users
    ├── Employees
    ├── Leads
    ├── Customers
    ├── Contacts
    ├── Forms
    ├── Webhooks
    ├── API Keys
    ├── Pipelines
    ├── Tasks
    ├── Activities
    └── Subscription
```

Semua business data harus memiliki `organization_id` atau mekanisme
tenant isolation yang setara.

------------------------------------------------------------------------

# 20. Tenant Context

Setiap authenticated request perlu memiliki tenant context.

Flow:

``` text
Request
   ↓
Authentication
   ↓
Identify User / API Key
   ↓
Identify Organization
   ↓
Tenant Context
   ↓
Service
   ↓
Repository
```

Contoh concept:

``` go
type TenantContext struct {
    OrganizationID uuid.UUID
    UserID         uuid.UUID
    Role           string
}
```

Query data harus selalu tenant-aware.

Contoh:

``` sql
SELECT *
FROM leads
WHERE organization_id = $1;
```

Tujuan utama:

> Data Organization A tidak boleh pernah terlihat atau termodifikasi
> oleh Organization B.

------------------------------------------------------------------------

# 21. Rate Limiting

Karena CRM menyediakan public API, rate limiting harus dipikirkan sejak
awal.

Flow:

``` text
API Request
    ↓
API Key Validation
    ↓
Rate Limiter
    ↓
Tenant Resolver
    ↓
Service
```

Contoh konsep:

``` text
Free:
100 requests/min

Starter:
1,000 requests/min

Business:
5,000 requests/min
```

Angka hanya contoh.

Limit sebenarnya harus disesuaikan dengan pricing dan infrastructure.

------------------------------------------------------------------------

# 22. Audit Log

CRM menyimpan data bisnis, sehingga audit log penting.

Contoh:

``` text
Budi
updated Lead #123
Status:
New → Contacted

Andi
assigned Lead #123
to:
Sinta

Admin
revoked API Key
crm_live_xxx
```

Audit log dapat mencatat:

-   actor
-   organization
-   action
-   entity
-   entity_id
-   old value
-   new value
-   timestamp
-   metadata

------------------------------------------------------------------------

# 23. Notification

Employee mobile membutuhkan notification.

Flow:

``` text
Lead Assigned
      ↓
Backend
      ↓
Notification Service
      ↓
FCM
      ↓
Flutter
```

Contoh:

``` text
🔔 New Lead

Budi Santoso has been assigned to you.
```

------------------------------------------------------------------------

# 24. Recommended Domain Structure

Backend Go monolith dapat memiliki struktur domain seperti:

``` text
crm_be/

cmd/
  api/

internal/

  auth/
  organization/
  user/
  employee/

  subscription/
  billing/

  api_key/
  integration/

  form/
  webhook/

  lead/
  customer/
  contact/

  pipeline/
  task/
  activity/

  notification/

  auditlog/

  middleware/
  database/
  storage/
```

Ini adalah logical organization, bukan berarti semua module harus dibuat
sekaligus.

------------------------------------------------------------------------

# 25. Monolith First

Walaupun produk nantinya mungkin besar, awalnya **jangan langsung
microservices**.

Gunakan:

``` text
Next.js
   ↓
Go Monolith
   ↓
PostgreSQL
```

Docker/Docker Compose untuk local development.

Jika scale meningkat, domain tertentu baru dapat dipisahkan menjadi
service.

Contoh future:

``` text
API Service
Lead Service
Notification Service
Integration Service
Report Service
```

Tetapi MVP tetap monolith.

------------------------------------------------------------------------

# 26. Domain Architecture

Core pipeline CRM:

``` text
CAPTURE
   ↓
LEAD
   ↓
ASSIGNMENT
   ↓
TASK
   ↓
FOLLOW UP
   ↓
CUSTOMER
   ↓
DEAL
   ↓
REVENUE
```

Integration layer:

``` text
API
Embedded Form
Webhook
```

berfungsi sebagai **Lead Capture Layer**.

Application layer:

``` text
Owner Dashboard
Employee Mobile
```

berfungsi sebagai **Lead Management Layer**.

------------------------------------------------------------------------

# 27. Product Structure

Secara umum:

``` text
                         CRM SaaS
                            │
          ┌─────────────────┼─────────────────┐
          │                 │                 │
          ▼                 ▼                 ▼
      Marketing          Dashboard            API
      Next.js             Next.js             Go
          │                 │                 │
      crm.com          app.crm.com       api.crm.com
                            │
                   ┌────────┼─────────┐
                   │        │         │
                  CRM    Billing    Developer
                                     │
                               ┌─────┼─────┐
                               │     │     │
                              API   Form  Webhook
                             Keys
```

Employee:

``` text
CRM Backend
     ↓
Flutter Employee App
```

------------------------------------------------------------------------

# 28. Suggested Development Roadmap

## Phase 0 --- Foundation

-   Go project setup
-   PostgreSQL
-   Docker
-   Configuration
-   Logging
-   Error handling
-   Database migration
-   Basic project architecture

## Phase 1 --- Multi-Tenant Auth

-   User
-   Organization
-   Membership
-   Owner
-   Registration
-   Login
-   JWT
-   Email verification
-   Tenant context
-   Basic RBAC

## Phase 2 --- Subscription

-   Subscription model
-   Free tier
-   Plan limits
-   Usage tracking
-   Billing abstraction
-   Subscription status

## Phase 3 --- API Key

-   API key creation
-   Hash storage
-   API key validation
-   Revoke
-   Disable
-   Last used tracking
-   Environment/name

## Phase 4 --- Lead API

-   Lead model
-   Create Lead API
-   Lead listing
-   Lead detail
-   Lead status
-   Source
-   Validation
-   Rate limiting

## Phase 5 --- Form Integration

-   Form model
-   Form fields
-   Form submission
-   Form ID
-   Embedded form
-   Form configuration

## Phase 6 --- Assignment & Task

-   Employee
-   Lead assignment
-   Manual assignment
-   Round robin
-   Task
-   Activity
-   Follow up

## Phase 7 --- Employee Mobile

-   Flutter setup
-   Login
-   Dashboard
-   My Leads
-   My Tasks
-   Lead detail
-   Activity
-   Notification
-   Profile

## Phase 8 --- Owner Dashboard

-   Dashboard
-   Lead management
-   Employee management
-   Assignment
-   Form management
-   API key management
-   Subscription
-   Usage
-   Reports

## Phase 9 --- Webhook

-   Inbound webhook
-   Outbound webhook
-   Event system
-   Webhook delivery
-   Retry
-   Delivery log

## Phase 10 --- Advanced CRM

-   Customer
-   Contact
-   Pipeline
-   Deal
-   Automation
-   Advanced analytics
-   Advanced reporting

------------------------------------------------------------------------

# 29. MVP Principle

Jangan membuat CRM menjadi terlalu besar sejak awal.

Core MVP:

``` text
Register
   ↓
Organization
   ↓
Owner
   ↓
Free Subscription
   ↓
API Key
   ↓
Lead API
   ↓
Lead
   ↓
Employee
   ↓
Assignment
   ↓
Flutter
   ↓
Task
   ↓
Follow Up
```

Kemudian:

``` text
Embedded Form
Webhook
Subscription Upgrade
Billing
Pipeline
Customer
Deal
Analytics
Automation
```

dapat dikembangkan setelah core workflow berjalan.

------------------------------------------------------------------------

# 30. Product Principle

Setiap fitur baru harus diuji dengan pertanyaan:

> "Apakah fitur ini membantu bisnis menangkap, mengelola,
> mendistribusikan, atau mengkonversi lead?"

Jika iya, fitur tersebut relevan dengan core CRM.

Jika tidak, jangan buru-buru memasukkannya ke MVP.

------------------------------------------------------------------------

# 31. Important Product Decisions

Keputusan yang sudah disepakati:

1.  CRM menggunakan **multi-tenant architecture sejak awal**.
2.  Backend menggunakan **Go monolith terlebih dahulu**, bukan
    microservices.
3.  Owner menggunakan **Next.js web dashboard**.
4.  Employee menggunakan **Flutter mobile app**.
5.  Landing page dan CRM dashboard dipisahkan secara konsep.
6.  Landing page fokus pada marketing, pricing, registration,
    documentation, dan login.
7.  Operational CRM dilakukan di dashboard.
8.  User melakukan registration dengan membuat organization.
9.  Email verification dilakukan setelah registration.
10. Setelah verification, user login ke owner dashboard.
11. Organization baru mendapatkan Free Tier.
12. API Key dibuat secara explicit dari dashboard.
13. API Key tidak dikirim melalui email.
14. API Key digunakan untuk authentication integration.
15. Form memiliki identifier sendiri, seperti Form ID.
16. Lead dapat masuk melalui API, Embedded Form, atau Webhook.
17. Semua source akhirnya menghasilkan model Lead yang sama.
18. Lead dapat di-assign ke employee.
19. Employee mengelola pekerjaan melalui Flutter.
20. Subscription dan billing dikelola melalui dashboard.
21. Role minimal: Owner, Admin, Manager, Employee.
22. Tenant isolation harus menjadi concern utama.
23. Public API harus memiliki rate limiting.
24. Audit Log disiapkan untuk aktivitas penting.
25. Outbound webhook dapat dikembangkan setelah inbound flow stabil.

------------------------------------------------------------------------

# 32. Open Questions for Future Brainstorming

Hal-hal berikut belum final dan perlu didiskusikan sebelum implementasi:

-   Nama produk
-   Target market utama
-   Pricing final
-   Free tier limits
-   Subscription provider / payment gateway
-   Email provider
-   API authentication mechanism final
-   API versioning
-   Webhook security/signature
-   Embedded form security
-   CORS strategy
-   Rate limit strategy
-   Usage metering
-   File storage
-   Push notification architecture
-   Employee invitation flow
-   RBAC detail
-   Lead status lifecycle
-   Pipeline design
-   Customer conversion flow
-   Data retention
-   Tenant deletion
-   Backup strategy
-   Observability
-   Deployment architecture
-   CI/CD strategy
-   Domain/subdomain structure
-   API documentation
-   Developer portal
-   Security model

------------------------------------------------------------------------

# 33. Core Mental Model

Produk ini dapat dipahami dengan sederhana:

``` text
                GET LEADS
                   │
        ┌──────────┼──────────┐
        │          │          │
       API       FORM      WEBHOOK
        │          │          │
        └──────────┼──────────┘
                   ▼
                 LEAD
                   │
                   ▼
              ASSIGNMENT
                   │
                   ▼
               EMPLOYEE
                   │
                   ▼
                 TASK
                   │
                   ▼
               FOLLOW UP
                   │
                   ▼
               CUSTOMER
                   │
                   ▼
                  DEAL
                   │
                   ▼
                REVENUE
```

**Landing page menjual sistem.**

**Dashboard mengoperasikan sistem.**

**API/Form/Webhook memasukkan data ke sistem.**

**Flutter membantu employee bekerja dari data tersebut.**

**Go backend menjadi pusat seluruh business logic dan tenant
isolation.**
