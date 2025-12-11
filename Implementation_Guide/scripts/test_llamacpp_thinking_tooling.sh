#!/bin/bash

# HelixCode - Llama.cpp Thinking & Tooling Test Script
# Tests local coding models with advanced reasoning and tool calling capabilities

echo "=================================================="
echo "HelixCode - Llama.cpp Thinking & Tooling Test"
echo "=================================================="

# Configuration
LLAMA_CPP_SERVER="http://localhost:8080"
TEST_MODELS=("codellama:7b" "codellama:13b" "llama3.1:8b" "deepseek-coder:6.7b")

# Check if Llama.cpp server is running
echo ""
echo "1. Checking Llama.cpp server..."
if ! curl -s "$LLAMA_CPP_SERVER/health" > /dev/null 2>&1; then
    echo "❌ Llama.cpp server not running at $LLAMA_CPP_SERVER"
    echo "   Please start the server: ./server -m models/codellama-7b.Q4_K_M.gguf"
    exit 1
else
    echo "✅ Llama.cpp server is running"
fi

# Check available models
echo ""
echo "2. Checking available models..."
AVAILABLE_MODELS=$(curl -s "$LLAMA_CPP_SERVER/tags" 2>/dev/null | jq -r '.models[].name' 2>/dev/null || echo "")

if [ -z "$AVAILABLE_MODELS" ]; then
    echo "❌ No models found in Llama.cpp server"
    echo "   Please load models into the server"
    exit 1
else
    echo "✅ Available models:"
    echo "$AVAILABLE_MODELS" | while read model; do
        echo "   - $model"
    done
fi

# Function to test reasoning capabilities
_test_reasoning_capabilities() {
    local model=$1
    local test_name=$2
    local prompt=$3
    
    echo ""
    echo "🧠 Testing $test_name with $model"
    
    # Create reasoning prompt
    REASONING_PROMPT="Think step by step about: $prompt. Show your reasoning process clearly."
    
    RESPONSE=$(curl -s -X POST "$LLAMA_CPP_SERVER/completion" \
        -H "Content-Type: application/json" \
        -d "{
            \"model\": \"$model\",
            \"prompt\": \"$REASONING_PROMPT\",
            \"stream\": false,
            \"temperature\": 0.3,
            \"max_tokens\": 1000
        }" | jq -r '.content' 2>/dev/null)
    
    if [ $? -ne 0 ] || [ -z "$RESPONSE" ]; then
        echo "   ❌ Reasoning test failed"
        return 1
    fi
    
    # Check for reasoning indicators
    if echo "$RESPONSE" | grep -i -E "step|first|then|next|therefore|conclusion" > /dev/null; then
        echo "   ✅ Reasoning capability detected"
        echo "   Response preview: $(echo "$RESPONSE" | head -3 | tr '\n' ' ')"
        return 0
    else
        echo "   ⚠️  Limited reasoning capability"
        echo "   Response: $(echo "$RESPONSE" | head -2)"
        return 1
    fi
}

# Function to test tool calling
_test_tool_calling() {
    local model=$1
    local test_name=$2
    local command=$3
    
    echo ""
    echo "🛠️  Testing $test_name with $model"
    
    # Create tool calling prompt
    TOOL_PROMPT="You have access to these tools:
- create_file: Create files with content
- run_tests: Execute test suites
- git_commit: Commit code changes

When you need to use a tool, respond with: TOOL: tool_name ARGS: json_arguments

User request: $command

Respond with tool calls if needed:"
    
    RESPONSE=$(curl -s -X POST "$LLAMA_CPP_SERVER/completion" \
        -H "Content-Type: application/json" \
        -d "{
            \"model\": \"$model\",
            \"prompt\": \"$TOOL_PROMPT\",
            \"stream\": false,
            \"temperature\": 0.7,
            \"max_tokens\": 500
        }" | jq -r '.content' 2>/dev/null)
    
    if [ $? -ne 0 ] || [ -z "$RESPONSE" ]; then
        echo "   ❌ Tool calling test failed"
        return 1
    fi
    
    # Check for tool call patterns
    if echo "$RESPONSE" | grep -i -E "TOOL:|create_file|run_tests|git_commit" > /dev/null; then
        echo "   ✅ Tool calling capability detected"
        echo "   Response: $(echo "$RESPONSE" | head -3)"
        return 0
    else
        echo "   ⚠️  Limited tool calling capability"
        return 1
    fi
}

# Function to test complex code generation
_test_complex_code_generation() {
    local model=$1
    local complexity=$2
    
    echo ""
    echo "💻 Testing $complexity code generation with $model"
    
    case $complexity in
        "simple")
            PROMPT="Create a Go function that reverses a string. Return only the code."
            ;;
        "medium")
            PROMPT="Create a Go HTTP middleware that logs requests and responses with timing. Return only the code."
            ;;
        "complex")
            PROMPT="Create a complete Go microservice with REST API, database connection, and error handling. Return only the code."
            ;;
    esac
    
    RESPONSE=$(curl -s -X POST "$LLAMA_CPP_SERVER/completion" \
        -H "Content-Type: application/json" \
        -d "{
            \"model\": \"$model\",
            \"prompt\": \"$PROMPT\",
            \"stream\": false,
            \"temperature\": 0.3,
            \"max_tokens\": 1500
        }" | jq -r '.content' 2>/dev/null)
    
    if [ $? -ne 0 ] || [ -z "$RESPONSE" ]; then
        echo "   ❌ Code generation failed"
        return 1
    fi
    
    # Check for Go code patterns
    if echo "$RESPONSE" | grep -E "package|import|func |type |struct " > /dev/null; then
        echo "   ✅ Go code generated successfully"
        
        # Try to extract and test compilation
        CODE=$(echo "$RESPONSE" | sed -n '/```go/,/```/p' | sed '1d;$d' | grep -v '^```')
        if [ -z "$CODE" ]; then
            CODE=$(echo "$RESPONSE" | grep -E '^(package|import|func|type)' | head -20)
        fi
        
        if [ -n "$CODE" ]; then
            TEMP_FILE="/tmp/test_$(date +%s).go"
            echo "$CODE" > "$TEMP_FILE"
            
            # Quick syntax check
            if go fmt "$TEMP_FILE" > /dev/null 2>&1; then
                echo "   ✅ Generated code passes go fmt"
                rm "$TEMP_FILE"
                return 0
            else
                echo "   ⚠️  Generated code has formatting issues"
                rm "$TEMP_FILE"
                return 1
            fi
        fi
        return 0
    else
        echo "   ❌ No valid Go code generated"
        return 1
    fi
}

# Main test execution
echo ""
echo "3. Testing models for thinking and tooling capabilities..."

SUCCESSFUL_MODELS=()

for MODEL in "${TEST_MODELS[@]}"; do
    # Check if model is available
    if ! echo "$AVAILABLE_MODELS" | grep -q "$MODEL"; then
        echo ""
        echo "⚠️  Skipping $MODEL - not available"
        continue
    fi
    
    echo ""
    echo "================================================================"
    echo "Testing: $MODEL"
    echo "================================================================"
    
    REASONING_PASS=0
    TOOL_PASS=0
    CODE_PASS=0
    
    # Test reasoning capabilities
    if _test_reasoning_capabilities "$MODEL" "Algorithm Design" "Design an algorithm to find duplicate files in a directory"; then
        REASONING_PASS=1
    fi
    
    if _test_reasoning_capabilities "$MODEL" "System Architecture" "Design a caching system for a web application"; then
        REASONING_PASS=$((REASONING_PASS + 1))
    fi
    
    # Test tool calling
    if _test_tool_calling "$MODEL" "File Operations" "Create a configuration file for a web server"; then
        TOOL_PASS=1
    fi
    
    if _test_tool_calling "$MODEL" "Git Operations" "Commit the current changes with a descriptive message"; then
        TOOL_PASS=$((TOOL_PASS + 1))
    fi
    
    # Test code generation
    if _test_complex_code_generation "$MODEL" "simple"; then
        CODE_PASS=1
    fi
    
    if _test_complex_code_generation "$MODEL" "medium"; then
        CODE_PASS=$((CODE_PASS + 1))
    fi
    
    # Evaluate model performance
    TOTAL_TESTS=$((REASONING_PASS + TOOL_PASS + CODE_PASS))
    if [ $TOTAL_TESTS -ge 4 ]; then
        echo ""
        echo "✅ $MODEL PASSED comprehensive testing"
        echo "   Reasoning: $REASONING_PASS/2, Tooling: $TOOL_PASS/2, Code: $CODE_PASS/2"
        SUCCESSFUL_MODELS+=("$MODEL")
    else
        echo ""
        echo "⚠️  $MODEL has limited capabilities"
        echo "   Reasoning: $REASONING_PASS/2, Tooling: $TOOL_PASS/2, Code: $CODE_PASS/2"
    fi
    
    # Add delay between model tests
    sleep 2
done

# Final results
echo ""
echo "=================================================="
echo "TEST RESULTS"
echo "=================================================="

if [ ${#SUCCESSFUL_MODELS[@]} -gt 0 ]; then
    echo "✅ SUCCESSFUL MODELS (Thinking + Tooling):"
    for model in "${SUCCESSFUL_MODELS[@]}"; do
        echo "   - $model"
    done
    
    echo ""
    echo "🎉 Llama.cpp integration ready for HelixCode!"
    echo "   ${#SUCCESSFUL_MODELS[@]} model(s) support advanced capabilities"
else
    echo "❌ No models fully support thinking and tooling"
    echo ""
    echo "RECOMMENDATIONS:"
    echo "1. Try larger models (13b+ variants)"
    echo "2. Use specialized coding models (CodeLlama, DeepSeek Coder)"
    echo "3. Fine-tune models for tool calling"
    exit 1
fi

# Recommendations for HelixCode implementation
echo ""
echo "IMPLEMENTATION RECOMMENDATIONS:"
echo "1. Use ${SUCCESSFUL_MODELS[0]} as default for development"
echo "2. Implement fallback to simpler models when needed"
echo "3. Add model-specific prompt tuning"
echo "4. Enable thinking mode for complex tasks"
echo "5. Implement tool calling for automation"

echo ""
echo "=================================================="
echo "✅ Llama.cpp Thinking & Tooling Test Complete"
echo "=================================================="

exit 0