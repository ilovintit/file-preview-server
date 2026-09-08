#!/usr/bin/env bash
set -euo pipefail
preview_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
source "$preview_root/.gitea/scripts/go-env.sh"
export XDG_CACHE_HOME="$preview_root/.cache"
preview_fixture_dir="$preview_root/internal/module/preview/testdata/text"
preview_profile="file://$preview_root/.cache/soffice-text-profile"
# Fixture authoring only. Actual PDF conversion validation runs in PR CI.
soffice "-env:UserInstallation=$preview_profile" --version
while IFS='|' read -r preview_ext preview_filter; do
  soffice "-env:UserInstallation=$preview_profile" --headless \
    --convert-to "$preview_ext:$preview_filter" --outdir "$preview_fixture_dir" \
    "$preview_fixture_dir/source.fodt"
  test -s "$preview_fixture_dir/source.$preview_ext"
done <<'FILTERS'
docm|Office Open XML Text VBA
dot|MS Word 97 Vorlage
dotx|Office Open XML Text Template
odt|writer8
ott|writer8_template
rtf|Rich Text Format
txt|Text (encoded):UTF8
xml|MS Word 2003 XML
uof|UOF text
FILTERS
