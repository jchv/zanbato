#!/usr/bin/env bash
# Validate Zanbato KSY/KST tests against the upstream Kaitai Struct compiler.
# This ensures our test definitions are correct before testing them against
# Zanbato's own compiler.
#
# Usage:
#   nix develop .#validate -c ./scripts/validate-upstream.sh
#
# Or, if you have JDK 17+ and sbt installed:
#   ./scripts/validate-upstream.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPILER_DIR="$REPO_DIR/internal/third_party/kaitai_struct_compiler"
TESTS_DIR="$REPO_DIR/internal/third_party/kaitai_struct_tests"
FORMATS_DIR="$REPO_DIR/internal/third_party/kaitai_struct_formats"
JAVA_RUNTIME_DIR="$REPO_DIR/internal/third_party/kaitai_struct_java_runtime"
CUSTOM_FORMATS_DIR="$REPO_DIR/testdata/formats"
CUSTOM_KST_DIR="$REPO_DIR/testdata/spec/ks"
CUSTOM_SRC_DIR="$REPO_DIR/testdata/src"
CUSTOM_NEGATIVE_DIR="$REPO_DIR/testdata/negative"

# Temp directory for build artifacts
WORK_DIR=$(mktemp -d)
trap "rm -rf $WORK_DIR" EXIT

download_file() {
    url="$1"
    dest="$2"

    curl --fail --location --silent --show-error "$url" --output "$dest"
    if [ ! -s "$dest" ]; then
        echo "ERROR: Downloaded empty file from $url"
        exit 1
    fi
}

echo "=== Checking prerequisites ==="
if ! command -v java &>/dev/null; then
    echo "ERROR: java not found. Run: nix develop .#validate -c $0"
    exit 1
fi
if ! command -v sbt &>/dev/null; then
    echo "ERROR: sbt not found. Run: nix develop .#validate -c $0"
    exit 1
fi
if ! command -v javac &>/dev/null; then
    echo "ERROR: javac not found. Run: nix develop .#validate -c $0"
    exit 1
fi
if ! command -v curl &>/dev/null; then
    echo "ERROR: curl not found. Run: nix develop .#validate -c $0"
    exit 1
fi
echo "  java: $(java -version 2>&1 | head -1)"
echo "  sbt: $(sbt --version 2>&1 | tail -1)"

# Check for custom test files
KSY_COUNT=$(ls "$CUSTOM_FORMATS_DIR"/*.ksy 2>/dev/null | wc -l)
KST_COUNT=$(ls "$CUSTOM_KST_DIR"/*.kst 2>/dev/null | wc -l)
echo "  Custom formats: $KSY_COUNT KSY files"
echo "  Custom tests:   $KST_COUNT KST files"
if [ "$KSY_COUNT" -eq 0 ]; then
    echo "No custom KSY files found in $CUSTOM_FORMATS_DIR"
    exit 0
fi
if [ "$KST_COUNT" -ne "$KSY_COUNT" ]; then
    echo "ERROR: Expected matching KSY/KST counts, got $KSY_COUNT KSY files and $KST_COUNT KST files."
    exit 1
fi

echo ""
echo "=== Step 1: Build upstream Kaitai Struct compiler ==="
COMPILER_BIN="$COMPILER_DIR/jvm/target/universal/stage/bin/kaitai-struct-compiler"
if [ -x "$COMPILER_BIN" ]; then
    echo "  Compiler already built, skipping."
else
    echo "  Building compiler (this may take a few minutes)..."
    (cd "$COMPILER_DIR" && sbt compilerJVM/stage)
fi

echo ""
echo "=== Step 2: Compile custom KSY files to Java ==="
COMPILED_DIR="$WORK_DIR/compiled/java/src/io/kaitai/struct/testformats"
mkdir -p "$COMPILED_DIR"
echo "  Compiling $KSY_COUNT KSY files..."
"$COMPILER_BIN" \
    --verbose file \
    -t java \
    --outdir "$WORK_DIR/compiled/java/src" \
    --import-path "$TESTS_DIR/formats" \
    --import-path "$TESTS_DIR/formats/ks_path" \
    --import-path "$CUSTOM_FORMATS_DIR" \
    --java-package io.kaitai.struct.testformats \
    "$CUSTOM_FORMATS_DIR"/*.ksy

COMPILED_COUNT=$(find "$WORK_DIR/compiled/java/src" -name "*.java" 2>/dev/null | wc -l)
echo "  Compiled $COMPILED_COUNT Java format files."
if [ "$COMPILED_COUNT" -ne "$KSY_COUNT" ]; then
    echo "ERROR: Compiled $COMPILED_COUNT Java format files, expected $KSY_COUNT."
    exit 1
fi

echo ""
echo "=== Step 3: Verify negative KSY files are rejected by upstream ==="
NEG_COUNT=0
if [ -d "$CUSTOM_NEGATIVE_DIR" ]; then
    NEG_COUNT=$(find "$CUSTOM_NEGATIVE_DIR" -maxdepth 1 -name "*.ksy" 2>/dev/null | wc -l)
fi
if [ "$NEG_COUNT" -gt 0 ]; then
    echo "  Found $NEG_COUNT negative KSY files; each must fail to compile."
    NEG_OUT="$WORK_DIR/negative-out"
    NEG_FAILED=0
    for neg in "$CUSTOM_NEGATIVE_DIR"/*.ksy; do
        base=$(basename "$neg" .ksy)
        rm -rf "$NEG_OUT"
        mkdir -p "$NEG_OUT"
        if "$COMPILER_BIN" \
                file \
                -t java \
                --outdir "$NEG_OUT" \
                --import-path "$TESTS_DIR/formats" \
                --import-path "$TESTS_DIR/formats/ks_path" \
                --import-path "$CUSTOM_FORMATS_DIR" \
                --java-package io.kaitai.struct.testformats \
                "$neg" >"$WORK_DIR/neg.log" 2>&1; then
            echo "ERROR: upstream compiled $base.ksy successfully; expected failure."
            cat "$WORK_DIR/neg.log"
            NEG_FAILED=$((NEG_FAILED + 1))
        else
            echo "  $base.ksy: upstream rejected (as expected)"
        fi
    done
    if [ "$NEG_FAILED" -gt 0 ]; then
        echo "ERROR: $NEG_FAILED negative file(s) were accepted by upstream."
        exit 1
    fi
fi

echo ""
echo "=== Step 4: Build upstream KST translator ==="
TRANSLATOR_DIR="$TESTS_DIR/translator"
# Ensure the compiler is published locally for the translator
echo "  Publishing compiler locally..."
(cd "$COMPILER_DIR" && sbt compilerJVM/publishLocal 2>&1 | tail -3)
echo "  Building translator..."
(cd "$TRANSLATOR_DIR" && sbt compile 2>&1 | tail -3)

echo ""
echo "=== Step 5: Translate custom KST files to Java tests ==="
# Create overlay directory mimicking upstream structure
OVERLAY_DIR="$WORK_DIR/overlay"
OVERLAY_TRANSLATOR_DIR="$OVERLAY_DIR/translator"
OVERLAY_SPEC_JAVA_DIR="$OVERLAY_DIR/spec/java"
OVERLAY_SIBLING_FORMATS_DIR="$WORK_DIR/formats"
mkdir -p "$OVERLAY_DIR/formats" "$OVERLAY_DIR/spec/ks" "$OVERLAY_DIR/src" "$OVERLAY_SPEC_JAVA_DIR/src/io/kaitai/struct/spec" "$OVERLAY_SIBLING_FORMATS_DIR" "$OVERLAY_TRANSLATOR_DIR"
ln -sfn "$TRANSLATOR_DIR/build.sbt" "$OVERLAY_TRANSLATOR_DIR/build.sbt"
ln -sfn "$TRANSLATOR_DIR/project" "$OVERLAY_TRANSLATOR_DIR/project"
ln -sfn "$TRANSLATOR_DIR/src" "$OVERLAY_TRANSLATOR_DIR/src"
# Symlink all upstream test formats + our custom formats where the translator loads test specs from
for f in "$TESTS_DIR/formats"/* "$TESTS_DIR/formats"/.[!.]* "$TESTS_DIR/formats"/..?*; do
    [ -e "$f" ] || continue
    ln -sfn "$f" "$OVERLAY_DIR/formats/" 2>/dev/null || true
done
for f in "$CUSTOM_FORMATS_DIR"/* "$CUSTOM_FORMATS_DIR"/.[!.]* "$CUSTOM_FORMATS_DIR"/..?*; do
    [ -e "$f" ] || continue
    ln -sfn "$f" "$OVERLAY_DIR/formats/" 2>/dev/null || true
done
# Symlink the full upstream Kaitai format repository + our custom formats where the translator expects imports
for f in "$FORMATS_DIR"/* "$FORMATS_DIR"/.[!.]* "$FORMATS_DIR"/..?*; do
    [ -e "$f" ] || continue
    ln -sfn "$f" "$OVERLAY_SIBLING_FORMATS_DIR/" 2>/dev/null || true
done
for f in "$CUSTOM_FORMATS_DIR"/* "$CUSTOM_FORMATS_DIR"/.[!.]* "$CUSTOM_FORMATS_DIR"/..?*; do
    [ -e "$f" ] || continue
    ln -sfn "$f" "$OVERLAY_SIBLING_FORMATS_DIR/" 2>/dev/null || true
done
# Copy upstream src + our custom src
for f in "$TESTS_DIR/src"/*; do
    ln -sf "$f" "$OVERLAY_DIR/src/" 2>/dev/null || true
done
for f in "$CUSTOM_SRC_DIR"/*; do
    ln -sf "$f" "$OVERLAY_DIR/src/" 2>/dev/null || true
done
# Copy custom KST files
for f in "$CUSTOM_KST_DIR"/*.kst; do
    ln -sf "$f" "$OVERLAY_DIR/spec/ks/" 2>/dev/null || true
done

# Get list of custom test names (without .kst extension)
CUSTOM_TESTS=""
for f in "$CUSTOM_KST_DIR"/*.kst; do
    name=$(basename "$f" .kst)
    CUSTOM_TESTS="$CUSTOM_TESTS $name"
done

echo "  Translating $KST_COUNT KST files to Java tests..."
TEST_OUT_DIR="$OVERLAY_SPEC_JAVA_DIR/src/io/kaitai/struct/spec"
TRANSLATOR_OUT_DIR="$OVERLAY_DIR/spec/ks/out/java/src/io/kaitai/struct/spec"
(cd "$OVERLAY_TRANSLATOR_DIR" && sbt "run -t java $CUSTOM_TESTS" 2>&1)

# Copy generated tests out of the upstream-compatible overlay
if [ -d "$TRANSLATOR_OUT_DIR" ]; then
    find "$TRANSLATOR_OUT_DIR" -name "*.java" -exec cp {} "$TEST_OUT_DIR/" \;
fi
TEST_COUNT=$(find "$TEST_OUT_DIR" -name "*.java" 2>/dev/null | wc -l)
echo "  Generated $TEST_COUNT Java test files."

if [ "$TEST_COUNT" -ne "$KST_COUNT" ]; then
    echo "ERROR: Generated $TEST_COUNT Java test files, expected $KST_COUNT."
    exit 1
fi

echo ""
echo "=== Step 6: Build Kaitai Java runtime ==="
if [ ! -d "$JAVA_RUNTIME_DIR/src/main/java" ]; then
    echo "ERROR: Kaitai Java runtime submodule not found at $JAVA_RUNTIME_DIR"
    echo "Run: git submodule update --init --recursive"
    exit 1
fi

RUNTIME_CLASS_DIR="$WORK_DIR/runtime/classes"
RUNTIME_SOURCES="$WORK_DIR/runtime_sources.txt"
mkdir -p "$RUNTIME_CLASS_DIR"
find "$JAVA_RUNTIME_DIR/src/main/java" -name "*.java" > "$RUNTIME_SOURCES"
javac -d "$RUNTIME_CLASS_DIR" @"$RUNTIME_SOURCES"

echo ""
echo "=== Step 7: Compile Java tests ==="

# Get TestNG and its runtime dependencies
if command -v cs &>/dev/null; then
    echo "  Resolving TestNG with coursier..."
    if ! TESTNG_JARS=$(cs fetch --classpath org.testng:testng:7.10.2 org.slf4j:slf4j-simple:2.0.16); then
        echo "ERROR: Failed to resolve TestNG with coursier."
        exit 1
    fi
elif command -v coursier &>/dev/null; then
    echo "  Resolving TestNG with coursier..."
    if ! TESTNG_JARS=$(coursier fetch --classpath org.testng:testng:7.10.2 org.slf4j:slf4j-simple:2.0.16); then
        echo "ERROR: Failed to resolve TestNG with coursier."
        exit 1
    fi
else
    TESTNG_JARS=""
    for dir in "$HOME/.ivy2" "$HOME/.cache/coursier"; do
        if [ -d "$dir" ]; then
            found_jars=$(find "$dir" \( -name "testng-*.jar" -o -name "jcommander-*.jar" -o -name "slf4j-api-*.jar" -o -name "slf4j-simple-*.jar" \) 2>/dev/null | paste -sd: -)
            if [ -n "$found_jars" ]; then
                if [ -n "$TESTNG_JARS" ]; then
                    TESTNG_JARS="$TESTNG_JARS:$found_jars"
                else
                    TESTNG_JARS="$found_jars"
                fi
            fi
        fi
    done
    if [ -z "$TESTNG_JARS" ]; then
        echo "  Downloading TestNG..."
        TESTNG_JAR="$WORK_DIR/testng.jar"
        JCOMMANDER_JAR="$WORK_DIR/jcommander.jar"
        SLF4J_API_JAR="$WORK_DIR/slf4j-api.jar"
        SLF4J_SIMPLE_JAR="$WORK_DIR/slf4j-simple.jar"
        download_file \
            "https://repo1.maven.org/maven2/org/testng/testng/7.10.2/testng-7.10.2.jar" \
            "$TESTNG_JAR"
        download_file \
            "https://repo1.maven.org/maven2/com/beust/jcommander/1.82/jcommander-1.82.jar" \
            "$JCOMMANDER_JAR"
        download_file \
            "https://repo1.maven.org/maven2/org/slf4j/slf4j-api/2.0.16/slf4j-api-2.0.16.jar" \
            "$SLF4J_API_JAR"
        download_file \
            "https://repo1.maven.org/maven2/org/slf4j/slf4j-simple/2.0.16/slf4j-simple-2.0.16.jar" \
            "$SLF4J_SIMPLE_JAR"
        TESTNG_JARS="$TESTNG_JAR:$JCOMMANDER_JAR:$SLF4J_API_JAR:$SLF4J_SIMPLE_JAR"
    fi
fi
if [ -z "$TESTNG_JARS" ]; then
    echo "ERROR: TestNG classpath is empty."
    exit 1
fi
IFS=':' read -r -a TESTNG_JAR_FILES <<< "$TESTNG_JARS"
for jar in "${TESTNG_JAR_FILES[@]}"; do
    if [ ! -s "$jar" ]; then
        echo "ERROR: TestNG dependency jar not found or empty: $jar"
        exit 1
    fi
done

# Copy the upstream CommonSpec.java
COMMON_SPEC="$TESTS_DIR/spec/java/src/io/kaitai/struct/spec/CommonSpec.java"
if [ -f "$COMMON_SPEC" ]; then
    cp "$COMMON_SPEC" "$TEST_OUT_DIR/"
fi

# Compile
CLASS_DIR="$WORK_DIR/classes"
mkdir -p "$CLASS_DIR"
CLASSPATH="$RUNTIME_CLASS_DIR:$TESTNG_JARS"

echo "  Compiling format classes..."
find "$WORK_DIR/compiled/java/src" -name "*.java" > "$WORK_DIR/format_sources.txt"
javac -cp "$CLASSPATH" -d "$CLASS_DIR" @"$WORK_DIR/format_sources.txt"

if [ "$TEST_COUNT" -gt 0 ]; then
    echo "  Compiling test classes..."
    find "$TEST_OUT_DIR" -name "*.java" > "$WORK_DIR/test_sources.txt"
    javac -cp "$CLASSPATH:$CLASS_DIR" -d "$CLASS_DIR" @"$WORK_DIR/test_sources.txt"
fi

echo ""
echo "=== Step 8: Run Java tests ==="
if [ "$TEST_COUNT" -gt 0 ]; then
    # Create TestNG config
    cat > "$WORK_DIR/testng.xml" <<TESTNG_EOF
<!DOCTYPE suite SYSTEM "https://testng.org/testng-1.0.dtd">
<suite name="Zanbato Custom Tests">
  <test name="spec">
    <packages>
      <package name="io.kaitai.struct.spec"/>
    </packages>
  </test>
</suite>
TESTNG_EOF

    cd "$OVERLAY_SPEC_JAVA_DIR"
    TESTNG_LOG="$WORK_DIR/testng.log"
    java -cp "$CLASSPATH:$CLASS_DIR" org.testng.TestNG -verbose 2 "$WORK_DIR/testng.xml" 2>&1 | tee "$TESTNG_LOG"
    if ! grep -q "Failures: 0" "$TESTNG_LOG"; then
        echo "ERROR: TestNG reported one or more failures."
        exit 1
    fi
fi

echo ""
echo "=== Summary ==="
echo "  KSY compilation: $COMPILED_COUNT/$KSY_COUNT succeeded"
echo "  KST translation: $TEST_COUNT/$KST_COUNT succeeded"
echo "  See above for Java test results."
