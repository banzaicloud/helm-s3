#!/usr/bin/env bash

set -eo pipefail

exit_code_success=0
exit_code_fatal=1

function download_file() {
    local -r downloader="${1}"
    local -r source="${2}"
    local -r destination="${3}"

    test ! -f "${destination}" ||
        log_fatal "destination file '%s' already exists" "${destination}"

    case "${downloader}" in
    curl)
        curl -sSL "${source}" -o "${destination}" || log_fatal "downloading '%s' failed" "${source}"
        ;;
    wget)
        wget -q "${source}" -O "${destination}" || log_fatal "downloading '%s' failed" "${source}"
        ;;
    *)
        log_fatal "unknown downloader: '%s'" "${downloader}"
        ;;
    esac
}

function get_architecture() {
    # shellcheck disable=SC2155 # Note: not using exit status.
    local architecture="$(uname -m)"

    case "${architecture}" in
    aarch64_be | aarch64 | armv6l | armv7l | armv8b | armv8l)
        architecture=arm64
        ;;
    x86_64)
        architecture=amd64
        ;;
    *) ;;
    esac

    printf "%s" "${architecture}"
}

function get_first_binary() {
    local -r candidates=("${@}")

    local binary=""
    for candidate in "${candidates[@]}"; do
        if which "${candidate}" &>/dev/null; then
            binary="${candidate}"

            break
        fi
    done

    if [ "${binary}" == "" ]; then
        log_fatal "required binary not found of candidates %s" "${candidates[*]}"
    fi

    printf "%s" "${binary}"
}

function log_fatal() {
    local -r format_string="${1}"
    local -r arguments=("${@:2}")

    # shellcheck disable=SC2059,SC2086
    printf >&2 "${format_string}\n" ${arguments[*]}

    exit ${exit_code_fatal}
}

function main() {
    local -r binary_path="bin/helms3"

    if [ -n "${HELM_PLUGIN_INSTALL_LOCAL}" ]; then
        if test -f "${binary_path}" && "${binary_path}" &>/dev/null; then
            exit ${exit_code_success} # Note: local plugin install with existing, working binary, nothing else to do.
        fi

        make build || log_fatal "building Helm S3 plugin locally failed"
        test -f "${binary_path}" || log_fatal "expected binary at '%s' cannot be found" "${binary_path}"
        ${binary_path} &>/dev/null || log_fatal "test running binary '%s' failed" "${binary_path}"

        exit ${exit_code_success} # Note: local plugin install with existing, working binary, nothing else to do.
    fi

    local -r checksum_verifier="$(get_first_binary shasum openssl sha512sum)"
    local -r downloader="$(get_first_binary curl wget)"
    local -r plugin_manifest_file_path="plugin.yaml"
    local -r project_name="helm-s3"
    local -r temporary_directory_path="$(mktemp -d)"
    local -r version_tag_prefix="v"
    #
    local -r repository_name="${project_name}"

    test -f "${plugin_manifest_file_path}" ||
        log_fatal "required plugin manifest file '%s' is missing" "${plugin_manifest_file_path}"

    local -r architecture="$(get_architecture)"
    local -r operating_system="$(uname | tr "[:upper:]" "[:lower:]")"
    local version

    version="$(awk '/^version:/ {print $2 ; exit}' "${plugin_manifest_file_path}")" ||
        log_fatal "required plugin manifest entry 'version' not found"

    grep -E -q "^\s+- ${architecture}$" "${plugin_manifest_file_path}" ||
        log_fatal "unsupported architecture: '%s'" "${architecture}"

    grep -E -q "^\s+- ${operating_system}$" "${plugin_manifest_file_path}" ||
        log_fatal "unsupported operating system '%s'" "${operating_system}"

    local -r download_url="https://github.com/banzaicloud/${repository_name}/releases/download"
    #
    local -r plugin_archive_name="${project_name}_${version}_${operating_system}_${architecture}"
    local -r plugin_checksums_file_name="${project_name}_${version}_sha512_checksums.txt"
    #
    local -r plugin_archive_file_name="${plugin_archive_name}.tar.gz"
    local -r plugin_checksums_download_path="${temporary_directory_path}/${plugin_checksums_file_name}"
    local -r plugin_checksums_url="${download_url}/${version_tag_prefix}${version}/${plugin_checksums_file_name}"
    #
    local -r plugin_archive_download_path="${temporary_directory_path}/${plugin_archive_file_name}"
    local -r plugin_archive_url="${download_url}/${version_tag_prefix}${version}/${plugin_archive_file_name}"

    download_file "${downloader}" "${plugin_archive_url}" "${plugin_archive_download_path}"
    download_file "${downloader}" "${plugin_checksums_url}" "${plugin_checksums_download_path}"

    verify_checksum "${checksum_verifier}" "${plugin_archive_download_path}" "${plugin_checksums_download_path}"

    mkdir -p "${temporary_directory_path}/${plugin_archive_name}"
    tar -xzf "${plugin_archive_download_path}" -C "${temporary_directory_path}/${plugin_archive_name}"
    mkdir -p "$(dirname "${binary_path}")"
    mv "${temporary_directory_path}/${plugin_archive_name}/${binary_path}" "${binary_path}"

    rm -fr "${temporary_directory_path}"
}

function verify_checksum() {
    local -r checker="${1}"
    local -r verified_file_path="${2}"
    local -r checksums_file_path="${3}"

    test -f "${verified_file_path}" || log_fatal "file '%s' to verify is missing" "${verified_file_path}"
    test -f "${checksums_file_path}" || log_fatal "checksums file '%s' to verify with is missing" "${checksums_file_path}"

    local -r verified_file_name="$(basename "${verified_file_path}")"
    local expected_checksum
    expected_checksum="$(awk "/^[0-9a-z]+  ${verified_file_name}$/ { print \$1 ; exit }" "${checksums_file_path}")" ||
        log_fatal "%s file not found in checksums file %s" "${verified_file_name}" "${checksums_file_path}"

    local actual_checksum=""
    case "${checker}" in
    openssl)
        actual_checksum="$(openssl dgst -sha512 "${verified_file_path}" | awk '{print $2}')"
        ;;
    sha512sum)
        actual_checksum="$(sha512sum "${verified_file_path}" | awk '{print $1}')"
        ;;
    shasum)
        actual_checksum="$(shasum -a 512 "${verified_file_path}" | awk '{print $1}')"
        ;;
    *)
        log_fatal "unknown checksum checker: %s" "${checker}"
        ;;
    esac

    test "${actual_checksum}" == "${expected_checksum}" ||
        log_fatal "'%s' file's checksum '%s' does not match recorded expected checksum '%s'" \
            "${verified_file_path}" "${actual_checksum}" "${expected_checksum}"
}

main "${@}"
