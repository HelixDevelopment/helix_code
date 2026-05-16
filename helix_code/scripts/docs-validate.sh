#!/bin/bash

# Documentation testing and validation script for HelixCode project
# This script validates documentation integrity, links, and code examples

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPORT_DIR="doc-reports"
TEMP_DIR=$(mktemp -d)
EXIT_CODE=0

echo -e "${BLUE}📚 HelixCode Documentation Validation${NC}"
echo "========================================="

# Create report directory
mkdir -p "${REPORT_DIR}"

# Clean previous reports
echo -e "${YELLOW}🧹 Cleaning previous documentation reports...${NC}"
rm -rf "${REPORT_DIR}"/*

# Initialize report
REPORT_FILE="${REPORT_DIR}/doc-validation-report-$(date +%Y%m%d-%H%M%S).md"
touch "${REPORT_FILE}"

cat > "${REPORT_FILE}" << EOF
# HelixCode Documentation Validation Report

Generated: $(date)
Status: Running...

## Validation Results

EOF

echo -e "${BLUE}🔍 Starting documentation validation...${NC}"

# 1. Check README.md
echo -e "${YELLOW}📖 Checking README.md...${NC}"
if [ -f "README.md" ]; then
    if [ -s "README.md" ]; then
        README_SIZE=$(wc -c < README.md)
        README_LINES=$(wc -l < README.md)
        echo -e "${GREEN}✅ README.md exists (${README_SIZE} bytes, ${README_LINES} lines)${NC}"
        echo "- README.md: ✅ EXISTS (${README_SIZE} bytes, ${README_LINES} lines)" >> "${REPORT_FILE}"
    else
        echo -e "${RED}❌ README.md exists but is empty${NC}"
        echo "- README.md: ❌ EMPTY" >> "${REPORT_FILE}"
        EXIT_CODE=1
    fi
else
    echo -e "${RED}❌ README.md missing${NC}"
    echo "- README.md: ❌ MISSING" >> "${REPORT_FILE}"
    EXIT_CODE=1
fi

# 2. Check for documentation files
echo -e "${YELLOW}📄 Scanning for documentation files...${NC}"
DOC_FILES=$(find . -name "*.md" -not -path "./vendor/*" -not -path "./.git/*" -not -path "./coverage-reports/*" -not -path "./doc-reports/*" | wc -l)
echo -e "${GREEN}✅ Found ${DOC_FILES} markdown documentation files${NC}"
echo "- Markdown files: ✅ ${DOC_FILES} files found" >> "${REPORT_FILE}"

# 3. Check for API documentation directories
echo -e "${YELLOW}🔌 Checking API documentation...${NC}"
API_DOC_FOUND=false
if [ -d "docs" ]; then
    echo -e "${GREEN}✅ docs/ directory exists${NC}"
    echo "- docs/ directory: ✅ EXISTS" >> "${REPORT_FILE}"
    API_DOC_FOUND=true
    
    # Check for common API doc patterns
    if [ -f "docs/api.md" ] || [ -f "docs/README.md" ] || find docs/ -name "*api*" -o -name "*swagger*" -o -name "*openapi*" | grep -q .; then
        echo -e "${GREEN}✅ API documentation structure found${NC}"
        echo "- API docs: ✅ FOUND" >> "${REPORT_FILE}"
    else
        echo -e "${YELLOW}⚠️ No specific API documentation found in docs/${NC}"
        echo "- API docs: ⚠️ NOT FOUND" >> "${REPORT_FILE}"
    fi
else
    echo -e "${YELLOW}⚠️ No docs/ directory found${NC}"
    echo "- docs/ directory: ⚠️ NOT FOUND" >> "${REPORT_FILE}"
fi

# 4. Validate internal links
echo -e "${YELLOW}🔗 Validating internal links...${NC}"
BROKEN_LINKS=0
TOTAL_LINKS=0

find . -name "*.md" -not -path "./vendor/*" -not -path "./.git/*" | while read -r file; do
    echo "  Checking $file..."
    
    # Extract markdown links
    grep -oE '\[.*\]\([^)]+\)' "$file" | while read -r link; do
        # Extract the path part
        path=$(echo "$link" | sed -E 's/\[.*\]\(([^)]+)\)/\1/')
        
        # Skip external URLs and anchor links
        if [[ "$path" == http* ]] || [[ "$path" == "#"* ]]; then
            continue
        fi
        
        # Remove URL fragments
        path=$(echo "$path" | cut -d'#' -f1)
        
        # Skip empty paths
        if [ -z "$path" ]; then
            continue
        fi
        
        ((TOTAL_LINKS++))
        
        # Check if the path exists relative to the file's directory
        DIR=$(dirname "$file")
        if [ -f "$DIR/$path" ] || [ -d "$DIR/$path" ]; then
            continue
        elif [ -f "$path" ] || [ -d "$path" ]; then
            continue
        else
            echo -e "${RED}❌ Broken link in $file: $path${NC}"
            ((BROKEN_LINKS++))
        fi
    done
done

if [ $BROKEN_LINKS -eq 0 ]; then
    echo -e "${GREEN}✅ All internal links are valid${NC}"
    echo "- Internal links: ✅ ALL VALID" >> "${REPORT_FILE}"
else
    echo -e "${RED}❌ Found $BROKEN_LINKS broken internal links${NC}"
    echo "- Internal links: ❌ $BROKEN_LINKS BROKEN" >> "${REPORT_FILE}"
    EXIT_CODE=1
fi

# 5. Validate code examples
echo -e "${YELLOW}💻 Validating Go code examples...${NC}"
CODE_EXAMPLES=0
INVALID_EXAMPLES=0

find . -name "*.md" -not -path "./vendor/*" -not -path "./.git/*" | while read -r file; do
    echo "  Checking Go examples in $file..."
    
    # Extract Go code blocks
    awk '/^```go$/,/^```$/' "$file" > "${TEMP_DIR}/go_code.tmp"
    
    # If we have Go code (not just the delimiters), try to validate it
    if [ $(wc -l < "${TEMP_DIR}/go_code.tmp") -gt 2 ]; then
        # Remove the ```go and ``` markers
        grep -v '^```$' "${TEMP_DIR}/go_code.tmp" | grep -v '^go$' > "${TEMP_DIR}/go_example.go"
        
        if [ -s "${TEMP_DIR}/go_example.go" ]; then
            ((CODE_EXAMPLES++))
            
            # Try gofmt to check syntax
            if gofmt -l "${TEMP_DIR}/go_example.go" > /dev/null 2>&1; then
                echo -e "${GREEN}✅ Valid Go example in $file${NC}"
            else
                echo -e "${RED}❌ Invalid Go example syntax in $file${NC}"
                ((INVALID_EXAMPLES++))
                EXIT_CODE=1
            fi
        fi
    fi
done

if [ $CODE_EXAMPLES -gt 0 ]; then
    if [ $INVALID_EXAMPLES -eq 0 ]; then
        echo -e "${GREEN}✅ All $CODE_EXAMPLES Go examples have valid syntax${NC}"
        echo "- Go examples: ✅ ALL VALID ($CODE_EXAMPLES examples)" >> "${REPORT_FILE}"
    else
        echo -e "${RED}❌ $INVALID_EXAMPLES out of $CODE_EXAMPLES Go examples have syntax errors${NC}"
        echo "- Go examples: ❌ $INVALID_EXAMPLES INVALID out of $CODE_EXAMPLES" >> "${REPORT_FILE}"
    fi
else
    echo -e "${YELLOW}⚠️ No Go code examples found${NC}"
    echo "- Go examples: ℹ️ NO EXAMPLES FOUND" >> "${REPORT_FILE}"
fi

# 6. Check for contributing guidelines
echo -e "${YELLOW}🤝 Checking for contributing guidelines...${NC}"
CONTRIB_FILES=("CONTRIBUTING.md" "CONTRIBUTING.rst" "docs/CONTRIBUTING.md" ".github/CONTRIBUTING.md")
CONTRIB_FOUND=false

for contrib_file in "${CONTRIB_FILES[@]}"; do
    if [ -f "$contrib_file" ] && [ -s "$contrib_file" ]; then
        echo -e "${GREEN}✅ Found contributing guidelines: $contrib_file${NC}"
        echo "- Contributing guidelines: ✅ FOUND ($contrib_file)" >> "${REPORT_FILE}"
        CONTRIB_FOUND=true
        break
    fi
done

if [ "$CONTRIB_FOUND" = false ]; then
    echo -e "${YELLOW}⚠️ No contributing guidelines found${NC}"
    echo "- Contributing guidelines: ⚠️ NOT FOUND" >> "${REPORT_FILE}"
fi

# 7. Check for license file
echo -e "${YELLOW}📜 Checking for license file...${NC}"
LICENSE_FILES=("LICENSE" "LICENSE.md" "LICENSE.txt" "COPYING")
LICENSE_FOUND=false

for license_file in "${LICENSE_FILES[@]}"; do
    if [ -f "$license_file" ] && [ -s "$license_file" ]; then
        echo -e "${GREEN}✅ Found license file: $license_file${NC}"
        echo "- License: ✅ FOUND ($license_file)" >> "${REPORT_FILE}"
        LICENSE_FOUND=true
        break
    fi
done

if [ "$LICENSE_FOUND" = false ]; then
    echo -e "${YELLOW}⚠️ No license file found${NC}"
    echo "- License: ⚠️ NOT FOUND" >> "${REPORT_FILE}"
fi

# 8. Check for change log
echo -e "${YELLOW}📝 Checking for changelog...${NC}"
CHANGELOG_FILES=("CHANGELOG.md" "CHANGES.md" "HISTORY.md" "NEWS.md")
CHANGELOG_FOUND=false

for changelog_file in "${CHANGELOG_FILES[@]}"; do
    if [ -f "$changelog_file" ] && [ -s "$changelog_file" ]; then
        echo -e "${GREEN}✅ Found changelog: $changelog_file${NC}"
        echo "- Changelog: ✅ FOUND ($changelog_file)" >> "${REPORT_FILE}"
        CHANGELOG_FOUND=true
        break
    fi
done

if [ "$CHANGELOG_FOUND" = false ]; then
    echo -e "${YELLOW}⚠️ No changelog found${NC}"
    echo "- Changelog: ⚠️ NOT FOUND" >> "${REPORT_FILE}"
fi

# 9. Check for package documentation
echo -e "${YELLOW}📦 Checking Go package documentation...${NC}"
UNDOCUMENTED_PACKAGES=0
TOTAL_PACKAGES=0

find ./internal ./cmd -name "*.go" -not -path "./vendor/*" | while read -r go_file; do
    DIR=$(dirname "$go_file")
    if [ -f "$DIR/*.go" ]; then
        ((TOTAL_PACKAGES++))
        
        # Check if package has documentation
        if ! grep -q "^// " "$DIR"/*.go 2>/dev/null; then
            echo -e "${YELLOW}⚠️ Package $DIR may lack documentation${NC}"
            ((UNDOCUMENTED_PACKAGES++))
        fi
    fi
done

# 10. Check for example usage
echo -e "${YELLOW}💡 Checking for examples...${NC}"
EXAMPLE_DIRS=("examples" "_examples" "example")
EXAMPLES_FOUND=false

for example_dir in "${EXAMPLE_DIRS[@]}"; do
    if [ -d "$example_dir" ]; then
        EXAMPLE_COUNT=$(find "$example_dir" -name "*.go" -o -name "*.md" | wc -l)
        if [ $EXAMPLE_COUNT -gt 0 ]; then
            echo -e "${GREEN}✅ Found examples in $example_dir/ ($EXAMPLE_COUNT examples)${NC}"
            echo "- Examples: ✅ FOUND ($EXAMPLE_COUNT in $example_dir/)" >> "${REPORT_FILE}"
            EXAMPLES_FOUND=true
            break
        fi
    fi
done

if [ "$EXAMPLES_FOUND" = false ]; then
    echo -e "${YELLOW}⚠️ No examples directory found${NC}"
    echo "- Examples: ⚠️ NOT FOUND" >> "${REPORT_FILE}"
fi

# 11. Finalize report
echo -e "${YELLOW}📋 Finalizing documentation validation report...${NC}"

# Update report status
if [ $EXIT_CODE -eq 0 ]; then
    STATUS="✅ PASSED"
    STATUS_COLOR="${GREEN}"
else
    STATUS="❌ FAILED"
    STATUS_COLOR="${RED}"
fi

# Create summary section
cat >> "${REPORT_FILE}" << EOF

## Summary

- **Overall Status**: ${STATUS}
- **Documentation Files**: ${DOC_FILES} files found
- **Internal Links**: $([ $BROKEN_LINKS -eq 0 ] && echo "All valid" || echo "$BROKEN_LINKS broken")
- **Code Examples**: $([ $CODE_EXAMPLES -gt 0 ] && echo "$CODE_EXAMPLES examples" || echo "None found")
- **API Documentation**: $([ "$API_DOC_FOUND" = true ] && echo "Available" || echo "Not found")

## Recommendations

EOF

# Add recommendations based on findings
if [ ! -f "README.md" ]; then
    echo "- Add a comprehensive README.md with project overview and quick start guide" >> "${REPORT_FILE}"
fi

if [ $BROKEN_LINKS -gt 0 ]; then
    echo "- Fix $BROKEN_LINKS broken internal links in documentation" >> "${REPORT_FILE}"
fi

if [ $INVALID_EXAMPLES -gt 0 ]; then
    echo "- Fix $INVALID_EXAMPLES Go code examples with syntax errors" >> "${REPORT_FILE}"
fi

if [ "$CONTRIB_FOUND" = false ]; then
    echo "- Add CONTRIBUTING.md with contribution guidelines and development setup" >> "${REPORT_FILE}"
fi

if [ "$LICENSE_FOUND" = false ]; then
    echo "- Add a LICENSE file to clarify usage terms" >> "${REPORT_FILE}"
fi

if [ "$EXAMPLES_FOUND" = false ]; then
    echo "- Create examples/ directory with usage examples and tutorials" >> "${REPORT_FILE}"
fi

if [ "$API_DOC_FOUND" = false ]; then
    echo "- Consider adding API documentation in docs/ directory" >> "${REPORT_FILE}"
fi

# Clean up temporary files
rm -rf "${TEMP_DIR}"

# Print summary
echo ""
echo -e "${BLUE}📊 Documentation Validation Summary${NC}"
echo "====================================="
echo -e "Status: ${STATUS_COLOR}${STATUS}${NC}"
echo -e "Documentation Files: ${DOC_FILES}"
echo -e "Internal Links: $([ $BROKEN_LINKS -eq 0 ] && echo "${GREEN}All valid${NC}" || echo "${RED}$BROKEN_LINKS broken${NC}")"
echo -e "Code Examples: $([ $INVALID_EXAMPLES -eq 0 ] && echo "${GREEN}All valid${NC}" || echo "${RED}$INVALID_EXAMPLES invalid${NC}")"
echo -e "Detailed Report: ${REPORT_FILE}"

echo ""
echo -e "${BLUE}🎯 Documentation Quality Score${NC}"
echo "==============================="

# Calculate quality score (simple scoring system)
SCORE=100
[ ! -f "README.md" ] && SCORE=$((SCORE - 20))
[ $BROKEN_LINKS -gt 0 ] && SCORE=$((SCORE - 15))
[ $INVALID_EXAMPLES -gt 0 ] && SCORE=$((SCORE - 10))
[ "$CONTRIB_FOUND" = false ] && SCORE=$((SCORE - 10))
[ "$LICENSE_FOUND" = false ] && SCORE=$((SCORE - 5))
[ "$EXAMPLES_FOUND" = false ] && SCORE=$((SCORE - 10))
[ "$API_DOC_FOUND" = false ] && SCORE=$((SCORE - 5))

if [ $SCORE -ge 90 ]; then
    SCORE_COLOR="${GREEN}"
elif [ $SCORE -ge 80 ]; then
    SCORE_COLOR="${YELLOW}"
else
    SCORE_COLOR="${RED}"
fi

echo -e "Score: ${SCORE_COLOR}${SCORE}/100${NC}"

# Add score to report
echo "" >> "${REPORT_FILE}"
echo "## Documentation Quality Score: ${SCORE}/100" >> "${REPORT_FILE}"

exit $EXIT_CODE