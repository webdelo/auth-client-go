#!/bin/bash
# Run Go tests with progress bar and table display
# Usage: test-runner.sh [--race] [--verbose] [--coverprofile=FILE] [--tags=TAGS] [TEST_PATH]
# TEST_PATH defaults to "./..." if not specified

set -o pipefail

RACE_FLAG=""
COVERAGE_FILE=""
VERBOSE_MODE=""
TAGS_FLAG=""
TEST_PATH="./..."
TITLE=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --race)
            RACE_FLAG="-race"
            shift
            ;;
        --verbose|-v)
            VERBOSE_MODE="1"
            shift
            ;;
        --coverprofile=*)
            COVERAGE_FILE="${1#*=}"
            shift
            ;;
        --tags=*)
            TAGS_FLAG="-tags=${1#*=}"
            shift
            ;;
        --title=*)
            TITLE="${1#*=}"
            shift
            ;;
        -*)
            # Unknown flag, skip
            shift
            ;;
        *)
            # Positional argument - test path
            TEST_PATH="$1"
            shift
            ;;
    esac
done

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

# Title header
if [ -n "$TITLE" ]; then
    echo -e "${BOLD}${CYAN}━━━ ${TITLE} ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
fi

# Mode description
MODE_DESC="Running tests"
if [ -n "$VERBOSE_MODE" ]; then
    MODE_DESC="$MODE_DESC (verbose)"
fi
if [ -n "$RACE_FLAG" ]; then
    MODE_DESC="$MODE_DESC with race detector"
fi
if [ -n "$COVERAGE_FILE" ]; then
    MODE_DESC="$MODE_DESC with coverage"
fi
if [ -n "$TAGS_FLAG" ]; then
    MODE_DESC="$MODE_DESC [${TAGS_FLAG#-tags=}]"
fi

echo -e "${CYAN}${MODE_DESC}...${NC}"
echo ""

# Build test flags
COVER_FLAG=""
if [ -n "$COVERAGE_FILE" ]; then
    COVER_FLAG="-coverprofile=$COVERAGE_FILE"
fi

# ─────────────────────────────────────────────────────────────────────────────
# Verbose mode: run standard go test -v first, then collect stats from cache
# ─────────────────────────────────────────────────────────────────────────────
if [ -n "$VERBOSE_MODE" ]; then
    # Run tests with standard verbose output
    go test -v $RACE_FLAG $COVER_FLAG $TAGS_FLAG $TEST_PATH
    TEST_EXIT=$?
    echo ""
fi

# ─────────────────────────────────────────────────────────────────────────────
# Collect stats via JSON (from cache if verbose already ran)
# ─────────────────────────────────────────────────────────────────────────────

# Auto-detect module name from go.mod (project-independent)
MODULE_NAME=$(grep '^module ' go.mod 2>/dev/null | awk '{print $2}' | head -1)
if [ -z "$MODULE_NAME" ]; then
    MODULE_NAME="unknown"
fi

TOTAL_PACKAGES=$(go list ./... 2>/dev/null | grep -v '/examples' | wc -l)
COMPLETED_PACKAGES=0
BAR_WIDTH=40

declare -A PKG_TESTS PKG_PASSED PKG_FAILED PKG_SKIPPED PKG_COVERAGE PKG_TIME
declare -a PKG_ORDER FAILED_TESTS_LIST
declare -A FAILED_TEST_OUTPUT

# Temp directory for test outputs and data files
TEST_OUTPUT_DIR=$(mktemp -d)
FAILED_TESTS_FILE="$TEST_OUTPUT_DIR/failed_tests.txt"
trap "rm -rf $TEST_OUTPUT_DIR" EXIT

draw_progress() {
    local percent=$1
    local current_pkg=$2

    # Cap percent at 100
    if [ "$percent" -gt 100 ]; then
        percent=100
    fi

    local filled=$((percent * BAR_WIDTH / 100))
    local empty=$((BAR_WIDTH - filled))

    local bar=""
    for ((i=0; i<filled; i++)); do bar+="█"; done
    for ((i=0; i<empty; i++)); do bar+="░"; done

    local max_pkg_len=35
    if [ ${#current_pkg} -gt $max_pkg_len ]; then
        current_pkg="${current_pkg:0:$((max_pkg_len-3))}..."
    fi

    printf "\r\033[K  ${CYAN}[${bar}]${NC} ${BOLD}%3d%%${NC}  ${DIM}%s${NC}" "$percent" "$current_pkg" >&2
}

# Initialize TEST_EXIT for non-verbose mode
if [ -z "$VERBOSE_MODE" ]; then
    TEST_EXIT=0
fi

while IFS= read -r line; do
    action=$(echo "$line" | jq -r '.Action // empty' 2>/dev/null)
    test_name=$(echo "$line" | jq -r '.Test // empty' 2>/dev/null)
    pkg=$(echo "$line" | jq -r '.Package // empty' 2>/dev/null)
    output=$(echo "$line" | jq -r '.Output // empty' 2>/dev/null)
    elapsed=$(echo "$line" | jq -r '.Elapsed // empty' 2>/dev/null)

    [ -z "$pkg" ] && continue
    [[ "$pkg" == *"/examples"* ]] && continue

    short_pkg=$(echo "$pkg" | sed "s|${MODULE_NAME}/||")

    if [ -z "${PKG_TESTS[$short_pkg]+x}" ]; then
        PKG_TESTS[$short_pkg]=0
        PKG_PASSED[$short_pkg]=0
        PKG_FAILED[$short_pkg]=0
        PKG_SKIPPED[$short_pkg]=0
        PKG_COVERAGE[$short_pkg]="-"
        PKG_TIME[$short_pkg]="0.000s"
        PKG_ORDER+=("$short_pkg")
    fi

    if [ -z "$test_name" ]; then
        if [ -n "$elapsed" ]; then
            formatted_time=$(printf "%.3fs" "$elapsed")
            PKG_TIME[$short_pkg]="$formatted_time"
        fi

        if [[ "$action" =~ ^(pass|fail|skip)$ ]]; then
            COMPLETED_PACKAGES=$((COMPLETED_PACKAGES + 1))

            # Show progress bar only in non-verbose mode
            if [ -z "$VERBOSE_MODE" ] && [ "$TOTAL_PACKAGES" -gt 0 ]; then
                PERCENT=$((COMPLETED_PACKAGES * 100 / TOTAL_PACKAGES))
                draw_progress $PERCENT "$short_pkg"
            fi

            if [ "$action" = "fail" ]; then
                TEST_EXIT=1
            fi
        fi
    fi

    if [ -n "$test_name" ]; then
        test_key="${short_pkg}::${test_name}"
        # Create safe filename from test key
        safe_file=$(echo "$test_key" | md5sum | cut -d' ' -f1)
        output_file="$TEST_OUTPUT_DIR/$safe_file"

        case "$action" in
            "run")
                PKG_TESTS[$short_pkg]=$((${PKG_TESTS[$short_pkg]} + 1))
                : > "$output_file"  # Create/truncate file
                ;;
            "output")
                # Accumulate output for this test
                # Note: bash $() strips trailing newlines, so we use echo to add one
                if [ -f "$output_file" ]; then
                    echo "$output" >> "$output_file"
                fi
                ;;
            "pass")
                PKG_PASSED[$short_pkg]=$((${PKG_PASSED[$short_pkg]} + 1))
                rm -f "$output_file"
                ;;
            "skip")
                PKG_SKIPPED[$short_pkg]=$((${PKG_SKIPPED[$short_pkg]} + 1))
                rm -f "$output_file"
                ;;
            "fail")
                PKG_FAILED[$short_pkg]=$((${PKG_FAILED[$short_pkg]} + 1))
                # Write failed test info to file for later processing
                echo "$test_key" >> "$FAILED_TESTS_FILE"
                if [ -f "$output_file" ]; then
                    # Save output with marker
                    cp "$output_file" "$TEST_OUTPUT_DIR/fail_${safe_file}.out"
                    echo "$test_key|fail_${safe_file}.out" >> "$TEST_OUTPUT_DIR/fail_mapping.txt"
                fi
                rm -f "$output_file"
                ;;
        esac
    fi

    if [[ "$output" =~ coverage:\ ([0-9.]+)% ]]; then
        PKG_COVERAGE[$short_pkg]="${BASH_REMATCH[1]}%"
    fi

done < <(go test -json $RACE_FLAG $COVER_FLAG $TAGS_FLAG $TEST_PATH 2>&1)

# Load failed tests from files (to handle bash subshell variable issue)
if [ -f "$FAILED_TESTS_FILE" ]; then
    while IFS= read -r test_key; do
        FAILED_TESTS_LIST+=("$test_key")
    done < "$FAILED_TESTS_FILE"
fi

if [ -f "$TEST_OUTPUT_DIR/fail_mapping.txt" ]; then
    while IFS='|' read -r test_key output_file; do
        if [ -f "$TEST_OUTPUT_DIR/$output_file" ]; then
            FAILED_TEST_OUTPUT["$test_key"]=$(cat "$TEST_OUTPUT_DIR/$output_file")
        fi
    done < "$TEST_OUTPUT_DIR/fail_mapping.txt"
fi

if [ -z "$VERBOSE_MODE" ]; then
    echo ""
fi
echo ""

# ─────────────────────────────────────────────────────────────────────────────
# Table Output
# ─────────────────────────────────────────────────────────────────────────────

COL_PKG=38
COL_TESTS=7
COL_PASS=7
COL_FAIL=7
COL_SKIP=7
COL_TIME=8
COL_COV=9

draw_line() {
    local left="$1" mid="$2" right="$3"
    printf "${DIM}${left}"
    printf '─%.0s' $(seq 1 $((COL_PKG + 2)))
    printf "${mid}"
    printf '─%.0s' $(seq 1 $((COL_TESTS + 2)))
    printf "${mid}"
    printf '─%.0s' $(seq 1 $((COL_PASS + 2)))
    printf "${mid}"
    printf '─%.0s' $(seq 1 $((COL_FAIL + 2)))
    printf "${mid}"
    printf '─%.0s' $(seq 1 $((COL_SKIP + 2)))
    printf "${mid}"
    printf '─%.0s' $(seq 1 $((COL_TIME + 2)))
    if [ -n "$COVERAGE_FILE" ]; then
        printf "${mid}"
        printf '─%.0s' $(seq 1 $((COL_COV + 2)))
    fi
    printf "${right}${NC}\n"
}

draw_line "┌" "┬" "┐"

printf "${DIM}│${NC} ${BOLD}%-${COL_PKG}s${NC} " "Package"
printf "${DIM}│${NC} ${BOLD}%${COL_TESTS}s${NC} " "Tests"
printf "${DIM}│${NC} ${BOLD}%${COL_PASS}s${NC} " "Passed"
printf "${DIM}│${NC} ${BOLD}%${COL_FAIL}s${NC} " "Failed"
printf "${DIM}│${NC} ${BOLD}%${COL_SKIP}s${NC} " "Skip"
printf "${DIM}│${NC} ${BOLD}%${COL_TIME}s${NC} " "Time"
if [ -n "$COVERAGE_FILE" ]; then
    printf "${DIM}│${NC} ${BOLD}%${COL_COV}s${NC} " "Coverage"
fi
printf "${DIM}│${NC}\n"

draw_line "├" "┼" "┤"

TOTAL_TESTS=0
TOTAL_PASSED=0
TOTAL_FAILED=0
TOTAL_SKIPPED=0

for pkg in "${PKG_ORDER[@]}"; do
    tests="${PKG_TESTS[$pkg]}"
    passed="${PKG_PASSED[$pkg]}"
    failed="${PKG_FAILED[$pkg]}"
    skipped="${PKG_SKIPPED[$pkg]}"
    coverage="${PKG_COVERAGE[$pkg]}"
    elapsed="${PKG_TIME[$pkg]}"

    TOTAL_TESTS=$((TOTAL_TESTS + tests))
    TOTAL_PASSED=$((TOTAL_PASSED + passed))
    TOTAL_FAILED=$((TOTAL_FAILED + failed))
    TOTAL_SKIPPED=$((TOTAL_SKIPPED + skipped))

    if [ ${#pkg} -gt $((COL_PKG - 2)) ]; then
        name_display="${pkg:0:$((COL_PKG - 5))}..."
    else
        name_display="$pkg"
    fi

    if [ "$failed" -gt 0 ]; then
        indicator="${RED}✘${NC}"
    else
        indicator="${GREEN}✔${NC}"
    fi

    printf "${DIM}│${NC} %b %-$((COL_PKG - 2))s " "$indicator" "$name_display"
    printf "${DIM}│${NC} %${COL_TESTS}s " "$tests"
    printf "${DIM}│${NC} ${GREEN}%${COL_PASS}s${NC} " "$passed"

    if [ "$failed" -gt 0 ]; then
        printf "${DIM}│${NC} ${RED}${BOLD}%${COL_FAIL}s${NC} " "$failed"
    else
        printf "${DIM}│${NC} ${DIM}%${COL_FAIL}s${NC} " "$failed"
    fi

    if [ "$skipped" -gt 0 ]; then
        printf "${DIM}│${NC} ${YELLOW}%${COL_SKIP}s${NC} " "$skipped"
    else
        printf "${DIM}│${NC} ${DIM}%${COL_SKIP}s${NC} " "$skipped"
    fi

    printf "${DIM}│${NC} ${DIM}%${COL_TIME}s${NC} " "$elapsed"

    if [ -n "$COVERAGE_FILE" ]; then
        if [ "$coverage" != "-" ]; then
            printf "${DIM}│${NC} ${CYAN}%${COL_COV}s${NC} " "$coverage"
        else
            printf "${DIM}│${NC} ${DIM}%${COL_COV}s${NC} " "$coverage"
        fi
    fi
    printf "${DIM}│${NC}\n"
done

draw_line "├" "┼" "┤"

printf "${DIM}│${NC} ${BOLD}%-${COL_PKG}s${NC} " "TOTAL"
printf "${DIM}│${NC} ${BOLD}%${COL_TESTS}s${NC} " "$TOTAL_TESTS"
printf "${DIM}│${NC} ${GREEN}${BOLD}%${COL_PASS}s${NC} " "$TOTAL_PASSED"

if [ "$TOTAL_FAILED" -gt 0 ]; then
    printf "${DIM}│${NC} ${RED}${BOLD}%${COL_FAIL}s${NC} " "$TOTAL_FAILED"
else
    printf "${DIM}│${NC} ${DIM}%${COL_FAIL}s${NC} " "$TOTAL_FAILED"
fi

if [ "$TOTAL_SKIPPED" -gt 0 ]; then
    printf "${DIM}│${NC} ${YELLOW}%${COL_SKIP}s${NC} " "$TOTAL_SKIPPED"
else
    printf "${DIM}│${NC} ${DIM}%${COL_SKIP}s${NC} " "$TOTAL_SKIPPED"
fi

printf "${DIM}│${NC} %${COL_TIME}s " ""

if [ -n "$COVERAGE_FILE" ]; then
    TOTAL_COV=$(go tool cover -func="$COVERAGE_FILE" 2>/dev/null | grep total | awk '{print $3}')
    printf "${DIM}│${NC} ${CYAN}${BOLD}%${COL_COV}s${NC} " "$TOTAL_COV"
fi
printf "${DIM}│${NC}\n"

draw_line "└" "┴" "┘"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
# Failed Tests Section (only in non-verbose mode)
# ─────────────────────────────────────────────────────────────────────────────

if [ -z "$VERBOSE_MODE" ] && [ ${#FAILED_TESTS_LIST[@]} -gt 0 ]; then
    echo -e "${RED}${BOLD}━━━ Failed Tests ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    for test_key in "${FAILED_TESTS_LIST[@]}"; do
        pkg="${test_key%%::*}"
        test="${test_key##*::}"

        echo -e "  ${RED}✘${NC} ${BOLD}${test}${NC}"
        echo -e "    ${DIM}Package: ${pkg}${NC}"

        if [ -n "${FAILED_TEST_OUTPUT[$test_key]}" ]; then
            echo -e "    ${DIM}Output:${NC}"
            # Filter out noise lines (=== RUN, === PAUSE, === CONT, --- FAIL)
            echo "${FAILED_TEST_OUTPUT[$test_key]}" | grep -v "^=== " | grep -v "^--- " | head -25 | sed 's/^/      /'
        fi
        echo ""
    done
fi

# ─────────────────────────────────────────────────────────────────────────────
# Final Summary
# ─────────────────────────────────────────────────────────────────────────────

echo -e "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Determine final status based on actual failed tests count
if [ "$TOTAL_FAILED" -gt 0 ]; then
    echo -e "${RED}${BOLD}  ✘ FAILED${NC}  │  Tests: ${TOTAL_TESTS}  │  Passed: ${GREEN}${TOTAL_PASSED}${NC}  │  Failed: ${RED}${BOLD}${TOTAL_FAILED}${NC}  │  Skipped: ${YELLOW}${TOTAL_SKIPPED}${NC}"
    FINAL_EXIT=1
else
    echo -e "${GREEN}${BOLD}  ✔ PASSED${NC}  │  Tests: ${TOTAL_TESTS}  │  Passed: ${GREEN}${TOTAL_PASSED}${NC}  │  Failed: ${DIM}${TOTAL_FAILED}${NC}  │  Skipped: ${YELLOW}${TOTAL_SKIPPED}${NC}"
    FINAL_EXIT=0
fi

echo -e "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ -n "$COVERAGE_FILE" ] && [ -f "$COVERAGE_FILE" ]; then
    echo -e ""
    echo -e "${CYAN}📁 Coverage profile: ${COVERAGE_FILE}${NC}"
fi

exit $FINAL_EXIT
