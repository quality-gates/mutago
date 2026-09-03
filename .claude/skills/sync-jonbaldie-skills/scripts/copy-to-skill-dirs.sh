#!/usr/bin/env bash

set -euo pipefail

usage() {
  printf 'Usage: %s <project-directory> <skill-directory> [<skill-directory> ...]\n' "$(basename "$0")" >&2
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

[[ $# -ge 2 ]] || {
  usage
  exit 2
}

for command_name in mkdir rm cp grep mv; do
  command -v "${command_name}" >/dev/null 2>&1 || die "missing required command: ${command_name}"
done

[[ -d "$1" ]] || die "project directory does not exist: $1"
project_root="$(cd "$1" && pwd -P)"
readonly project_root
canonical_skills_dir="${project_root}/.agents/skills"
readonly canonical_manifest="${project_root}/.agents/sync-jonbaldie-skills.manifest"
shift

[[ -d "${canonical_skills_dir}" ]] || die "canonical skills directory is missing; run sync-skills.sh first"
[[ -f "${canonical_manifest}" ]] || die "managed-skill manifest is missing; run sync-skills.sh first"
canonical_skills_dir="$(cd "${canonical_skills_dir}" && pwd -P)"
readonly canonical_skills_dir

copy_to_directory() {
  local requested_directory="$1"
  local destination
  local destination_manifest
  local name

  if [[ "${requested_directory}" = /* ]]; then
    destination="${requested_directory}"
  else
    destination="${project_root}/${requested_directory}"
  fi

  mkdir -p "${destination}"
  destination="$(cd "${destination}" && pwd -P)"

  if [[ "${destination}" == "${canonical_skills_dir}" ]]; then
    printf 'Already canonical: %s\n' "${destination}"
    return
  fi

  destination_manifest="${destination}/.sync-jonbaldie-skills.manifest"

  if [[ -f "${destination_manifest}" ]]; then
    while IFS= read -r name; do
      [[ -n "${name}" && "${name}" != \#* ]] || continue
      [[ "${name}" =~ ^[a-z0-9][a-z0-9._-]*$ ]] || continue
      if ! grep -Fqx "${name}" "${canonical_manifest}"; then
        rm -rf "${destination:?}/${name}"
      fi
    done <"${destination_manifest}"
  fi

  while IFS= read -r name; do
    [[ -n "${name}" && "${name}" != \#* ]] || continue
    [[ "${name}" =~ ^[a-z0-9][a-z0-9._-]*$ ]] || die "unsafe skill name in manifest: ${name}"
    [[ -d "${canonical_skills_dir}/${name}" ]] || die "managed skill is missing: ${canonical_skills_dir}/${name}"
    rm -rf "${destination:?}/${name}"
    cp -a "${canonical_skills_dir}/${name}" "${destination}/${name}"
    printf '  copied %s -> %s\n' "${name}" "${destination}"
  done <"${canonical_manifest}"

  cp "${canonical_manifest}" "${destination}/.sync-jonbaldie-skills.manifest.tmp"
  mv "${destination}/.sync-jonbaldie-skills.manifest.tmp" "${destination_manifest}"
  printf 'Additional destination: %s\n' "${destination}"
}

for requested_directory in "$@"; do
  copy_to_directory "${requested_directory}"
done
