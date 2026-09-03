#!/usr/bin/env bash

set -euo pipefail

readonly MATTPOCOCK_REPO_URL="${MATTPOCOCK_SKILLS_REPO:-https://github.com/mattpocock/skills.git}"
readonly JONBALDIE_REPO_URL="${JONBALDIE_SKILLS_REPO:-https://github.com/jonbaldie/skills.git}"

usage() {
  printf 'Usage: %s <project-directory>\n' "$(basename "$0")" >&2
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

[[ $# -eq 1 ]] || {
  usage
  exit 2
}

for command_name in git find awk dirname basename mkdir rm cp sort grep mktemp mv; do
  command -v "${command_name}" >/dev/null 2>&1 || die "missing required command: ${command_name}"
done

[[ -d "$1" ]] || die "project directory does not exist: $1"
project_root="$(cd "$1" && pwd -P)"
readonly project_root
readonly agent_root="${project_root}/.agents"
readonly skills_dir="${agent_root}/skills"
readonly manifest="${agent_root}/sync-jonbaldie-skills.manifest"
temporary_root="$(mktemp -d "${TMPDIR:-/tmp}/sync-jonbaldie-skills.XXXXXX")"
readonly temporary_root
readonly names_file="${temporary_root}/managed-names"
readonly sorted_names_file="${temporary_root}/managed-names.sorted"

cleanup() {
  rm -rf "${temporary_root}"
}
trap cleanup EXIT

skill_name() {
  local skill_directory="$1"
  local name

  name="$(
    awk '
      BEGIN { in_frontmatter = 0 }
      /^---[[:space:]]*$/ {
        if (in_frontmatter == 0) { in_frontmatter = 1; next }
        exit
      }
      in_frontmatter && /^name:[[:space:]]*/ {
        sub(/^name:[[:space:]]*/, "")
        gsub(/^[[:space:]]+|[[:space:]]+$/, "")
        gsub(/^['\''"]|['\''"]$/, "")
        print
        exit
      }
    ' "${skill_directory}/SKILL.md"
  )"

  [[ -n "${name}" ]] || name="$(basename "${skill_directory}")"
  [[ "${name}" =~ ^[a-z0-9][a-z0-9._-]*$ ]] ||
    die "unsafe skill name '${name}' in ${skill_directory}/SKILL.md"
  printf '%s\n' "${name}"
}

copy_skill() {
  local source_directory="$1"
  local name="$2"
  local destination="${skills_dir}/${name}"

  rm -rf "${destination}"
  cp -a "${source_directory}" "${destination}"
  printf '%s\n' "${name}" >>"${names_file}"
  printf '  installed %s\n' "${name}"
}

install_collection() {
  local repository_root="$1"
  local skill_file
  local skill_directory
  local name
  local count=0

  while IFS= read -r -d '' skill_file; do
    skill_directory="$(dirname "${skill_file}")"
    name="$(skill_name "${skill_directory}")"
    copy_skill "${skill_directory}" "${name}"
    count=$((count + 1))
  done < <(
    find "${repository_root}/skills" \
      \( -name node_modules -o -name .git -o -name deprecated -o -name .agents -o -name .claude -o -name .gemini \) -prune -o \
      -type f -name SKILL.md -print0
  )

  [[ ${count} -gt 0 ]] || die "no skills found in ${repository_root}"
}

printf 'Fetching mattpocock/skills...\n'
GIT_TERMINAL_PROMPT=0 git clone --depth 1 --quiet "${MATTPOCOCK_REPO_URL}" "${temporary_root}/mattpocock"
printf 'Fetching jonbaldie/skills...\n'
GIT_TERMINAL_PROMPT=0 git clone --depth 1 --quiet "${JONBALDIE_REPO_URL}" "${temporary_root}/jonbaldie"

mattpocock_commit="$(git -C "${temporary_root}/mattpocock" rev-parse HEAD)"
readonly mattpocock_commit
jonbaldie_commit="$(git -C "${temporary_root}/jonbaldie" rev-parse HEAD)"
readonly jonbaldie_commit

mkdir -p "${skills_dir}"
: >"${names_file}"

printf 'Installing mattpocock/skills...\n'
install_collection "${temporary_root}/mattpocock"
printf 'Installing jonbaldie/skills...\n'
install_collection "${temporary_root}/jonbaldie"

sort -u "${names_file}" >"${sorted_names_file}"

if [[ -f "${manifest}" ]]; then
  while IFS= read -r old_name; do
    [[ -n "${old_name}" && "${old_name}" != \#* ]] || continue
    [[ "${old_name}" =~ ^[a-z0-9][a-z0-9._-]*$ ]] || continue
    if ! grep -Fqx "${old_name}" "${sorted_names_file}"; then
      rm -rf "${skills_dir:?}/${old_name}"
      printf '  removed retired skill %s\n' "${old_name}"
    fi
  done <"${manifest}"
fi

{
  printf '# mattpocock/skills %s\n' "${mattpocock_commit}"
  printf '# jonbaldie/skills %s\n' "${jonbaldie_commit}"
  while IFS= read -r managed_name; do
    printf '%s\n' "${managed_name}"
  done <"${sorted_names_file}"
} >"${temporary_root}/manifest"
cp "${temporary_root}/manifest" "${agent_root}/.sync-jonbaldie-skills.manifest.tmp"
mv "${agent_root}/.sync-jonbaldie-skills.manifest.tmp" "${manifest}"

printf 'mattpocock/skills commit: %s\n' "${mattpocock_commit}"
printf 'jonbaldie/skills commit: %s\n' "${jonbaldie_commit}"
printf 'Canonical destination: %s\n' "${skills_dir}"
