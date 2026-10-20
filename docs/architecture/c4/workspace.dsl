workspace "BFE Chat Service" "C4 views of approved GRACE architecture and merged Issue #25" {
    model {
        user = person "BFE user" "Uses conversations and messages."
        employee = person "BFE employee" "Communicates with authorized participants."
        admin = person "BFE administrator" "Manages authorized conversation operations."

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

        user -> clients "Uses"
        employee -> clients "Uses"
        admin -> clients "Uses"
        clients -> chat.ingress "Chat API and realtime" "HTTPS / WSS, DPoP"
        chat.app -> drive "Validate session and DPoP" "UDS"
        clients -> drive "Open or download documents under Drive ACL" "HTTPS"
    }
    views {
        systemContext chat "c1-system-context" {
            include *
            autolayout lr
        }
        container chat "c2-containers" {
            include *
            autolayout lr
        }
        component chat.app "c3-chat-components" {
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
