#!/bin/bash

# Shared helpers for deploy.sh and update.sh. Callers provide the
# DOCKER_COMPOSE array and COMPOSE_FILE path.

compose() {
    "${DOCKER_COMPOSE[@]}" -f "$COMPOSE_FILE" "$@"
}

wait_for_postgres() {
    local max_attempts="${1:-30}"
    local attempt

    for ((attempt = 1; attempt <= max_attempts; attempt++)); do
        if compose exec -T postgres sh -ec 'pg_isready -q -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-${POSTGRES_USER:-postgres}}"' > /dev/null 2>&1; then
            return 0
        fi
        sleep 2
    done

    echo "PostgreSQL did not become ready after $((max_attempts * 2)) seconds" >&2
    compose logs --tail=50 postgres >&2 || true
    return 1
}

wait_for_service() {
    local service="$1"
    local max_attempts="${2:-45}"
    local attempt container_id state

    for ((attempt = 1; attempt <= max_attempts; attempt++)); do
        container_id="$(compose ps -q "$service" 2>/dev/null || true)"
        if [ -n "$container_id" ]; then
            state="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container_id" 2>/dev/null || true)"
            case "$state" in
                healthy|running)
                    return 0
                    ;;
                unhealthy|dead|exited)
                    echo "Service ${service} entered state ${state}" >&2
                    compose logs --tail=50 "$service" >&2 || true
                    return 1
                    ;;
            esac
        fi
        sleep 2
    done

    echo "Service ${service} did not become ready after $((max_attempts * 2)) seconds" >&2
    compose logs --tail=50 "$service" >&2 || true
    return 1
}

backup_postgres() {
    local output_path="$1"

    if ! compose exec -T postgres sh -ec 'exec pg_dump --format=custom --no-owner --no-privileges -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-${POSTGRES_USER:-postgres}}"' > "$output_path"; then
        rm -f -- "$output_path"
        return 1
    fi
    if [ ! -s "$output_path" ]; then
        echo "PostgreSQL backup is empty: ${output_path}" >&2
        rm -f -- "$output_path"
        return 1
    fi
}

service_image() {
    local service="$1"
    local container_id

    container_id="$(compose ps -q "$service" 2>/dev/null || true)"
    if [ -z "$container_id" ]; then
        printf '%s\n' "not-running"
        return 0
    fi
    docker inspect --format '{{.Config.Image}}' "$container_id" 2>/dev/null || printf '%s\n' "unknown"
}

service_image_id() {
    local service="$1"
    local container_id

    container_id="$(compose ps -q "$service" 2>/dev/null || true)"
    if [ -z "$container_id" ]; then
        printf '%s\n' "not-running"
        return 0
    fi
    docker inspect --format '{{.Image}}' "$container_id" 2>/dev/null || printf '%s\n' "unknown"
}

service_is_running() {
    local service="$1"
    local container_id

    container_id="$(compose ps -q "$service" 2>/dev/null || true)"
    [ -n "$container_id" ] || return 1
    [ "$(docker inspect --format '{{.State.Running}}' "$container_id" 2>/dev/null || true)" = "true" ]
}
