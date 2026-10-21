workspace "BFE Chat Service" "GRACE architecture views: requirements, technology and module contracts" {
    !identifiers hierarchical

    model {
        user = person "BFE user" "Account holder; may become group creator, owner or member (ACT-USER, ACT25-USER, ACT25-OWNER, ACT25-MEMBER)."
        employee = person "BFE employee" "BFE account assigned to legacy conversations (ACT-EMPLOYEE)."
        admin = person "BFE administrator" "Uses the Admin UI for legacy assignments and read-only auditing (ACT-ADMIN)."

        clients = softwareSystem "BFE client applications" "Flutter Web/Android and separate administrative UI (EXT-CLIENTS, EXT-ADMIN-UI)." {
            tags "External"
        }
        drive = softwareSystem "BFE Drive / Internal Auth" "Owns BFE identity, session status and documents; Chat asks Internal Auth to validate sessions via UDS (EXT-AUTH, EXT-DRIVE)." {
            tags "External"
        }

        chat = softwareSystem "BFE Chat Service" "Legacy assigned conversations and Issue #25 direct/group conversations; owns membership, messages, visibility and history." {
            ingress = container "Chat HTTPS ingress" "Forwards Chat HTTPS and WebSocket traffic to loopback Go listener. Test VPS Nginx configuration." "Nginx" {
                tags "Infrastructure"
            }
            app = container "Chat Go service" "One Go executable; net/http, pgx, goose and embedded Centrifuge." "Go" {
                transport = component "Transport — MC-08 / MC25-08" "Routes /v1 legacy, /v2 conversations and /ws; validates request envelopes." "internal/transport, internal/issue25/http.go"
                auth = component "Identity and access — MC-01 / MC-04 / MC25-01" "Validates BFE sessions with DPoP/UDS; checks current chat-specific permissions." "internal/authentication, internal/authorization, internal/issue25"
                legacy = component "Legacy assignments and conversations — MC-02 / MC-03" "Maintains explicitly assigned legacy owner conversations." "internal/assignments, internal/conversations"
                lifecycle = component "Direct/group lifecycle — MC25-02" "Creates canonical direct pairs and groups; manages personal hide state." "internal/issue25"
                membership = component "Membership and roles — MC25-03" "Manages membership intervals, epochs, owner transfer and role changes." "internal/issue25"
                messages = component "Messages and history — MC25-04" "Commits ordered messages, idempotent send and membership-scoped history." "internal/issue25"
                realtime = component "Realtime — MC-05 / MC25-05" "Embedded Centrifuge with per-epoch authorized content-free wake-up delivery." "internal/realtime, internal/issue25"
                read = component "Read state — MC25-06" "Tracks per-member monotonic read markers and unread counts." "internal/issue25"
                transition = component "Legacy compatibility — MC25-07" "Preserves original legacy assignments and messages; controlled transition only." "internal/issue25, internal/storage"
                documents = component "Document metadata — MC-06 / MC25-09" "Stores document ID/title snapshots; never grants Drive document permissions." "internal/documentcontext, internal/issue25"
                persistence = component "Chat persistence — MC-07" "Chat-owned SQL transactions and goose startup migrations; no Drive table access." "internal/storage, internal/issue25"

                transport -> auth "Authenticate request"
                transport -> legacy "Legacy API"
                transport -> lifecycle "Conversation operations"
                transport -> membership "Membership/role operations"
                transport -> messages "Send and history"
                transport -> read "Read markers"
                auth -> drive "Validate BFE session and DPoP" "Unix domain socket"
                auth -> persistence "Resolve Chat-specific access"
                legacy -> persistence "Legacy data"
                lifecycle -> membership "Initialize membership"
                lifecycle -> persistence "Persist conversation and visibility"
                membership -> persistence "Commit role and epoch changes"
                messages -> membership "Check membership intervals"
                messages -> documents "Validate document snapshot"
                messages -> persistence "Commit before notification"
                messages -> realtime "Post-commit wake-up"
                read -> persistence "Commit read marker"
                transition -> persistence "Preserve legacy provenance"
                realtime -> auth "Validate subscriber"
                realtime -> membership "Check current membership epoch"
            }
            database = container "bfe_chat database" "Stores legacy and Issue #25 conversations, membership intervals, messages and read markers." "PostgreSQL 18" {
                tags "Database"
            }

            ingress -> app "Forward HTTP / WebSocket" "Loopback :8081"
            app -> database "Query / transaction / migration" "PostgreSQL/pgx/goose"
        }

        user -> clients "Chat and document actions"
        employee -> clients "Assigned legacy conversations"
        admin -> clients "Administrative operations"
        clients -> chat.ingress "Send Chat API/WS requests" "HTTPS/WSS, DPoP"
        clients -> drive "Open/download documents under Drive ACL" "HTTPS"
    }

    views {
        systemContext chat "requirements-c1-context" {
            description "RequirementsAnalysis.xml: ACT-USER/EMPLOYEE/ADMIN and ACT25 user roles. All people use external client applications; no direct Person-to-service access."
            include *
            autolayout lr
        }
        container chat "technology-development-c2-containers" {
            description "Technology.xml + DevelopmentPlan.xml: one Go service, embedded Centrifuge, PostgreSQL and separately owned Drive Internal Auth."
            include *
            autolayout lr
        }
        component chat.app "contracts-c3-components" {
            description "Contracts.xml: legacy MC-01..MC-08 and Issue #25 MC25-01..MC25-09 mapped to logical code responsibilities, not independent processes."
            include *
            autolayout lr
        }
        styles {
            element "External" {
                background "#666666"
                color "#ffffff"
            }
            element "Database" {
                shape Cylinder
            }
        }
    }
}
