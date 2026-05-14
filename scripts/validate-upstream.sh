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
CUSTOM_FORMATS_DIR="$REPO_DIR/testdata/formats"
CUSTOM_KST_DIR="$REPO_DIR/testdata/spec/ks"
CUSTOM_SRC_DIR="$REPO_DIR/testdata/src"

# Temp directory for build artifacts
WORK_DIR=$(mktemp -d)
trap "rm -rf $WORK_DIR" EXIT

echo "=== Checking prerequisites ==="
if ! command -v java &>/dev/null; then
    echo "ERROR: java not found. Run: nix develop .#validate -c $0"
    exit 1
fi
if ! command -v sbt &>/dev/null; then
    echo "ERROR: sbt not found. Run: nix develop .#validate -c $0"
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
    -d "$WORK_DIR/compiled/java/src" \
    --import-path "$TESTS_DIR/formats" \
    --import-path "$TESTS_DIR/formats/ks_path" \
    --import-path "$CUSTOM_FORMATS_DIR" \
    --java-package io.kaitai.struct.testformats \
    "$CUSTOM_FORMATS_DIR"/*.ksy || {
        echo "  WARNING: Some KSY files failed to compile (continuing with what succeeded)"
    }

COMPILED_COUNT=$(find "$WORK_DIR/compiled/java/src" -name "*.java" 2>/dev/null | wc -l)
echo "  Compiled $COMPILED_COUNT Java format files."
if [ "$COMPILED_COUNT" -eq 0 ]; then
    echo "ERROR: No Java files were compiled."
    exit 1
fi

echo ""
echo "=== Step 3: Build upstream KST translator ==="
TRANSLATOR_DIR="$TESTS_DIR/translator"
# Ensure the compiler is published locally for the translator
echo "  Publishing compiler locally..."
(cd "$COMPILER_DIR" && sbt compilerJVM/publishLocal 2>&1 | tail -3)
echo "  Building translator..."
(cd "$TRANSLATOR_DIR" && sbt compile 2>&1 | tail -3)

echo ""
echo "=== Step 4: Translate custom KST files to Java tests ==="
# Create overlay directory mimicking upstream structure
OVERLAY_DIR="$WORK_DIR/overlay"
mkdir -p "$OVERLAY_DIR/formats" "$OVERLAY_DIR/spec/ks" "$OVERLAY_DIR/src"
# Symlink upstream formats + our custom formats
for f in "$TESTS_DIR/formats"/*.ksy; do
    ln -sf "$f" "$OVERLAY_DIR/formats/" 2>/dev/null || true
done
for f in "$CUSTOM_FORMATS_DIR"/*.ksy; do
    ln -sf "$f" "$OVERLAY_DIR/formats/" 2>/dev/null || true
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
TEST_OUT_DIR="$WORK_DIR/tests/java/src/io/kaitai/struct/spec"
mkdir -p "$TEST_OUT_DIR"
(cd "$TRANSLATOR_DIR" && sbt "run -t java -d $WORK_DIR/test_out $CUSTOM_TESTS" 2>&1) || {
    echo "  WARNING: Some KST files failed to translate"
}

# Move generated tests
if [ -d "$WORK_DIR/test_out/java" ]; then
    find "$WORK_DIR/test_out/java" -name "*.java" -exec cp {} "$TEST_OUT_DIR/" \;
fi
TEST_COUNT=$(find "$TEST_OUT_DIR" -name "*.java" 2>/dev/null | wc -l)
echo "  Generated $TEST_COUNT Java test files."

if [ "$TEST_COUNT" -eq 0 ]; then
    echo "WARNING: No Java test files were generated. This may indicate translator issues."
    echo "Attempting direct compilation validation only..."
fi

echo ""
echo "=== Step 5: Compile Java tests ==="
# Get the Kaitai runtime jar
RUNTIME_JAR=$(find ~/.ivy2 ~/.cache/coursier -name "kaitai-struct-runtime-*.jar" 2>/dev/null | head -1)
if [ -z "$RUNTIME_JAR" ]; then
    echo "  Downloading Kaitai runtime..."
    RUNTIME_JAR="$WORK_DIR/kaitai-struct-runtime.jar"
    # Use the upstream runtime from Maven
    curl -sL "https://repo1.maven.org/maven2/io/kaitai/kaitai-struct-runtime/0.10/kaitai-struct-runtime-0.10.jar" \
        -o "$RUNTIME_JAR" 2>/dev/null || {
            echo "WARNING: Could not download runtime. Skipping Java test compilation."
            echo ""
            echo "=== Summary ==="
            echo "  KSY compilation: $COMPILED_COUNT/$KSY_COUNT succeeded"
            echo "  KST translation: $TEST_COUNT/$KST_COUNT succeeded"
            echo "  Java tests: SKIPPED (missing runtime)"
            exit 0
        }
fi

# Get TestNG jar
TESTNG_JAR=$(find ~/.ivy2 ~/.cache/coursier -name "testng-*.jar" 2>/dev/null | head -1)
if [ -z "$TESTNG_JAR" ]; then
    echo "WARNING: TestNG not found. Skipping Java test execution."
    echo ""
    echo "=== Summary ==="
    echo "  KSY compilation: $COMPILED_COUNT/$KSY_COUNT succeeded"
    echo "  KST translation: $TEST_COUNT/$KST_COUNT succeeded"
    echo "  Java tests: SKIPPED (missing TestNG)"
    exit 0
fi

# Copy the upstream CommonSpec.java
COMMON_SPEC="$TESTS_DIR/spec/java/src/io/kaitai/struct/spec/CommonSpec.java"
if [ -f "$COMMON_SPEC" ]; then
    cp "$COMMON_SPEC" "$TEST_OUT_DIR/"
fi

# Compile
CLASS_DIR="$WORK_DIR/classes"
mkdir -p "$CLASS_DIR"
CLASSPATH="$RUNTIME_JAR:$TESTNG_JAR"

echo "  Compiling format classes..."
find "$WORK_DIR/compiled/java/src" -name "*.java" > "$WORK_DIR/format_sources.txt"
javac -cp "$CLASSPATH" -d "$CLASS_DIR" @"$WORK_DIR/format_sources.txt" 2>&1 || {
    echo "WARNING: Some format classes failed to compile."
}

if [ "$TEST_COUNT" -gt 0 ]; then
    echo "  Compiling test classes..."
    find "$TEST_OUT_DIR" -name "*.java" > "$WORK_DIR/test_sources.txt"
    javac -cp "$CLASSPATH:$CLASS_DIR" -d "$CLASS_DIR" @"$WORK_DIR/test_sources.txt" 2>&1 || {
        echo "WARNING: Some test classes failed to compile."
    }
fi

echo ""
echo "=== Step 6: Run Java tests ==="
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

    cd "$OVERLAY_DIR"
    java -cp "$CLASSPATH:$CLASS_DIR" org.testng.TestNG "$WORK_DIR/testng.xml" 2>&1 || {
        echo ""
        echo "Some tests FAILED."
    }
fi

echo ""
echo "=== Summary ==="
echo "  KSY compilation: $COMPILED_COUNT/$KSY_COUNT succeeded"
echo "  KST translation: $TEST_COUNT/$KST_COUNT succeeded"
echo "  See above for Java test results."
