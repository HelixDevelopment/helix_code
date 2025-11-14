# HelixCode Test Challenges

This directory contains test challenges to validate HelixCode's code generation capabilities across different approaches and scenarios.

## Structure

- `test_framework/` - Testing infrastructure and validation scripts
- `approach_single_model/` - Single model approach implementation
- `approach_multiple_models/` - Multiple models approach (planned)
- `approach_distributed_work/` - Distributed work approach (planned)
- `approach_hybrid/` - Hybrid approach (planned)

## Test Framework

The test framework provides automated validation of generated code:

- Compilation testing
- Unit test execution
- Integration testing
- Performance benchmarking

### Running Tests

```bash
# Run all tests
./test_framework/test_runner.sh all

# Run specific test types
./test_framework/test_runner.sh unit
./test_framework/test_runner.sh integration
./test_framework/test_runner.sh e2e
```

## Current Status

- ✅ Single model approach: In progress
- ⏳ Multiple models approach: Planned
- ⏳ Distributed work approach: Planned
- ⏳ Hybrid approach: Planned

## Validation Criteria

Generated code must:
1. Compile without errors
2. Pass all unit tests
3. Meet functional requirements
4. Follow coding standards
5. Include proper documentation