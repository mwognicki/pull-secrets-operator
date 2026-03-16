#!/usr/bin/env bash

load_dotenv_file() {
  local dotenv_file="$1"

  if [[ -f "${dotenv_file}" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "${dotenv_file}"
    set +a
  fi
}

load_default_dotenv_files() {
  local root_dir="$1"

  load_dotenv_file "${root_dir}/.env"
  load_dotenv_file "${root_dir}/.env.local"
}
