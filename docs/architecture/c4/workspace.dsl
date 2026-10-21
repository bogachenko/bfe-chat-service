workspace "BFE Chat Service" "C4 views of approved GRACE architecture and merged Issue #25" {
    !identifiers hierarchical
    model {
        user = person "BFE user" "Uses conversations and messages."
        employee = person "BFE employee" "Communicates with authorized participants."
        admin = person "BFE administrator" "Legacy assignments and administrative history access (ACT-ADMIN)."
        groupOwner = person "Group owner" "Creates groups, transfers ownership and manages group roles (ACT25-OWNER)."
        groupMember = person "Group member" "Participates during membership intervals (ACT25-MEMBER)."

        drive = softwareSystem "BFE Drive / Internal Auth" "Provides session identity and DPoP verification via UDS; owns documents and document ACL." {
            tags "External"
        }
        clients = softwareSystem "BFE clients" "Web, Android and administrative interfaces; separately owned." {
            tags "External"
        }
        chat = softwareSystem "BFE Chat Service" "Durable personal and group conversations." {
            ingress = container "HTTPS ingress (Nginx)" "Routes /v1, /v2 and /ws to the loopback listener." "Nginx" {
                tags "Infrastructure"
            }
            app = container "Chat application" "HTTP/WebSocket API, chat domain services, embedded Centrifuge and migration bootstrap." "Go, net/http, Centrifuge" {
                transport = component "HTTP and WebSocket transport" "Routes legacy /v1 and Issue #25 /v2 requests and Centrifuge /ws." "internal/transport, internal/issue25/http.go"
                auth = component "Identity and authorization" "DPoP, UDS validation, current assignment or membership/role checks." "internal/authentication, internal/authorization, internal/issue25"
                legacy = component "Legacy conversations and assignments" "Legacy conversation lifecycle, authorization and messages." "internal/assignments, internal/conversations"
                lifecycle = component "Conversation and membership lifecycle" "Issue #25 direct/group lifecycle, epochs and role transitions." "internal/issue25"
                messages = component "Messaging and history" "Durable ordered messages, history visibility and read state." "internal/issue25, internal/conversations"
                realtime = component "Realtime delivery" "Embedded Centrifuge, authorized subscriptions and content-free wake-up notifications." "internal/realtime, internal/issue25"
                documents = component "Document context" "Validates and persists document ID/title snapshots; no document ACL." "internal/documentcontext"
                persistence = component "PostgreSQL adapter and migrations" "Transactionally stores Chat data, goose migrations at startup." "internal/storage, internal/issue25, pgx, goose"
                transport -> auth "Authenticate/authorize"
                transport -> legacy "Legacy operations"
                transport -> lifecycle "Conversation/member operations"
                transport -> messages "Send/history/read operations"
                legacy -> persistence "Persist legacy data"
                lifecycle -> persistence "Commit membership/roles"
                messages -> persistence "Commit messages/read states"
                messages -> documents "Validate document metadata"
                messages -> realtime "Request wake-up after commit"
                realtime -> auth "Verify subscribers"
                realtime -> persistence "Validate membership epochs"
                auth -> persistence "Read Chat authorization data"
                auth -> drive "Validate session + DPoP" "Unix domain socket"
            }
            database = container "bfe_chat" "Chat-owned database: legacy and Issue #25 tables, membership intervals, messages and read markers." "PostgreSQL 18" {
                tags "Database"
            }
            ingress -> app "Proxy HTTP/WS" "Loopback HTTP :8081"
            app -> database "Queries and transactional writes" "pgx / SQL"
        }

        user -> clients "Direct chats, messaging and history (REQ25-DIRECT, REQ25-SEND, REQ25-HISTORY)"
        groupOwner -> clients "Group creation and roles (REQ25-GROUP, REQ25-ROLES)"
        groupMember -> clients "Group participation (REQ25-MEMBERSHIP)"
        employee -> clients "Legacy assigned conversations (BP-SEND, BP-HISTORY)"
        admin -> clients "Legacy assignments and administrative read (BP-ASSIGN, BP-ADMIN-READ)"
        clients -> chat.ingress "Chat API and realtime" "HTTPS / WSS, DPoP"
        clients -> drive "Open or download documents under Drive ACL" "HTTPS"
    }
    views {
        systemContext chat "requirements-c1-context" {
            description "GRACE Stage 1 — RequirementsAnalysis.xml: actors ACT-USER, ACT-EMPLOYEE, ACT-ADMIN, ACT25-OWNER, ACT25-MEMBER; use cases REQ25-DIRECT/GROUP/ROLES/SEND/HISTORY. Shows logical user interactions, not new network entrypoints."
            include *
            autolayout lr
        }
        container chat "technology-development-c2-containers" {
            description "GRACE Stages 2–3 — Technology.xml and DevelopmentPlan.xml: approved runtime boundaries and deployed Chat components."
            include *
            autolayout lr
        }
        component chat.app "contracts-c3-components" {
            description "GRACE Stage 4 — Contracts.xml: module contracts MC-01..MC-08 and MC25-01..MC25-09 represented as logical Go components."
            include *
            autolayout lr
        }
        styles {
            element "External" {
                background "#777777"
                color "#ffffff"
            }
            element "Database" {
                shape Cylinder
            }
        }
        theme default
    }
}
