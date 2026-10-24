workspace "BFE Chat Service" "GRACE architecture views: requirements, technology and module contracts" {
    !identifiers hierarchical

    model {
        user = person "BFE user" "Account holder; may become group creator, owner or member (ACT-USER, ACT25-USER, ACT25-OWNER, ACT25-MEMBER)."
        employee = person "BFE employee" "BFE account assigned to legacy conversations (ACT-EMPLOYEE)."
        admin = person "BFE administrator" "Uses the Admin UI for legacy assignments and read-only auditing (ACT-ADMIN)."

        chatConsumers = softwareSystem "External Chat consumers" "Consumers of Chat Service APIs; implementation and transport migration are outside this service-only requirements revision (EXT-CHAT-CONSUMERS)." {
            tags "External"
        }
        adminUI = softwareSystem "BFE administrative UI" "External administrative interface for legacy assignments and read-only auditing; implementation is outside this service-only requirements revision (EXT-ADMIN-UI)." {
            tags "External"
        }
        ntfy = softwareSystem "ntfy" "Proposed target realtime wake-up transport for RA36-001. Chat Service publishes minimal wake-up signals; ntfy owns no message history or Chat authorization. Centrifuge remains effective until Human Approval." {
            tags "External"
        }
        drive = softwareSystem "BFE Drive / Internal Auth" "Owns BFE identity, session status and documents; Chat asks Internal Auth to validate sessions via UDS (EXT-AUTH, EXT-DRIVE)." {
            tags "External"
        }

        chat = softwareSystem "BFE Chat Service" "Owns membership, messages, visibility and history. Current approved realtime baseline is Centrifuge; proposed RA36-001 target is ntfy pending Human Approval." {
            ingress = container "Chat HTTPS ingress" "Forwards Chat HTTPS and WebSocket traffic to loopback Go listener. Test VPS Nginx configuration." "Nginx" {
                tags "Infrastructure"
            }
            app = container "Chat Go service" "One Go executable. Current approved baseline embeds Centrifuge; proposed RA36-001 target replaces it with ntfy pending Human Approval." "Go" {
                transport = component "Transport — MC-08 / MC25-08" "Routes /v1 legacy, /v2 conversations and /ws; validates request envelopes." "internal/transport, internal/issue25/http.go"
                auth = component "Identity and access — MC-01 / MC-04 / MC25-01" "Validates BFE sessions with DPoP/UDS; checks current chat-specific permissions." "internal/authentication, internal/authorization, internal/issue25"
                legacy = component "Legacy assignments and conversations — MC-02 / MC-03" "Maintains explicitly assigned legacy owner conversations." "internal/assignments, internal/conversations"
                lifecycle = component "Direct/group lifecycle — MC25-02" "Creates canonical direct pairs and groups; manages personal hide state." "internal/issue25"
                membership = component "Membership and roles — MC25-03" "Manages membership intervals, epochs, owner transfer and role changes." "internal/issue25"
                messages = component "Messages and history — MC25-04" "Commits ordered messages, idempotent send and membership-scoped history." "internal/issue25"
                realtime = component "Realtime — MC-05 / MC25-05" "Current baseline: embedded Centrifuge. Proposed target: ntfy publish of opaque wake-up envelopes; backend history remains authoritative." "internal/realtime, internal/issue25"
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
                realtime -> ntfy "Publish opaque wake-up envelope (proposed RA36-001 target only)"
            }
            database = container "bfe_chat database" "Stores legacy and Issue #25 conversations, membership intervals, messages and read markers." "PostgreSQL 18" {
                tags "Database"
            }

            ingress -> app "Forward HTTP / WebSocket" "Loopback :8081"
            app -> database "Query / transaction / migration" "PostgreSQL/pgx/goose"
        }

        user -> chatConsumers "Chat and document actions"
        employee -> chatConsumers "Assigned legacy conversations"
        admin -> adminUI "Administrative operations"
        chatConsumers -> chat.ingress "Send Chat API requests" "HTTPS, DPoP"
        chatConsumers -> drive "Open/download documents under Drive ACL" "HTTPS"
        
        adminUI -> chat.ingress "Administrative API operations" "HTTPS, DPoP"
    }

    views {
        systemContext chat "requirements-c1-context" {
            description "Service-only view of BFE Chat Service and its direct dependencies. External consumer implementations and their subscription wiring are outside this revision. ntfy is shown only as the proposed service-side publication dependency and is not effective before Human Approval."
            include *
            autolayout lr
        }
        container chat "technology-development-c2-containers" {
            description "Current approved ISSUE32 baseline: one Go service with embedded Centrifuge, PostgreSQL and Drive Internal Auth. Proposed RA36-001 target replaces Centrifuge with ntfy only after Human Approval."
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
