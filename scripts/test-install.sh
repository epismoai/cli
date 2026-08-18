#!/bin/sh
set -eu

test_root=$(mktemp -d "${TMPDIR:-/tmp}/epismo-install-test.XXXXXX")
case "$test_root" in "${TMPDIR:-/tmp}"/epismo-install-test.*|/tmp/epismo-install-test.*) ;; *) exit 1 ;; esac
cleanup() { rm -rf -- "$test_root"; }
trap cleanup EXIT INT TERM

mock_bin="$test_root/bin"
install_dir="$test_root/install"
mkdir -p "$mock_bin"

make_mock() {
  name="$1"
  shift
  script="$mock_bin/$name"
  printf '%s\n' '#!/bin/sh' "$@" > "$script"
  chmod +x "$script"
}

make_mock uname 'case "$1" in -s) echo Linux ;; -m) echo x86_64 ;; *) exit 1 ;; esac'
make_mock curl '
output=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) output="$2"; shift 2 ;;
    http*) url="$1"; shift ;;
    *) shift ;;
  esac
done
case "$url" in
  */checksums.txt) printf "%s  %s\n" test-checksum epismo_linux_amd64.tar.gz > "$output" ;;
  */epismo_linux_amd64.tar.gz) printf archive > "$output" ;;
  *) exit 1 ;;
esac'
make_mock sha256sum 'printf "%s  %s\n" test-checksum "$1"'
make_mock tar '
destination=""
while [ "$#" -gt 0 ]; do
  case "$1" in -C) destination="$2"; shift 2 ;; *) shift ;; esac
done
printf "%s\n" "#!/bin/sh" "echo 1.2.3" > "$destination/epismo"
chmod +x "$destination/epismo"'
make_mock install '
while [ "$#" -gt 2 ]; do shift; done
cp "$1" "$2"
chmod +x "$2"'

PATH="$mock_bin:$PATH" \
EPISMO_INSTALL_DIR="$install_dir" \
EPISMO_RELEASE_BASE_URL="https://releases.example.test" \
sh "$(dirname "$0")/../install.sh"

test -x "$install_dir/epismo"
test -f "$install_dir/epismo.install.json"
grep '"method": "curl"' "$install_dir/epismo.install.json" >/dev/null
grep '"installedVersion": "1.2.3"' "$install_dir/epismo.install.json" >/dev/null
