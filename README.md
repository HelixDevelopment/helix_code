![HelixCode - Distributed AI Development Platform](assets/Wide_Black.png)

# HelixCode - Distributed AI Development Platform

**Version**: 1.0.0  
**Package**: `dev.helix.code`  
**License**: MIT

HelixCode is an enterprise-grade distributed AI development platform that enables intelligent task division, work preservation, and cross-platform development workflows. Built with Go and designed for scalability, HelixCode provides a robust foundation for distributed computing with automatic checkpointing, rollback functionality, and real-time monitoring.

## 🚀 Key Features

### ✅ **Phase 1: Foundation** (Completed)
- **Database Schema**: Complete PostgreSQL schema with 11 tables for distributed computing
- **Authentication System**: JWT-based auth with session management
- **Worker Management**: Distributed worker registration and health monitoring
- **Task Management**: Intelligent task division with work preservation
- **Logo Integration**: Automatic asset generation with color extraction
- **REST API**: Comprehensive HTTP API with Gin framework
- **Configuration System**: Flexible config with environment variables

### ✅ **Phase 2: Core Services** (Completed)
- **Advanced Task Division**: Intelligent task splitting with dependency management
- **LLM Provider Integration**: Multi-provider support (Llama.cpp, Ollama, OpenAI)
- **Distributed Computing**: Work preservation with automatic checkpointing
- **MCP Protocol**: Model Context Protocol implementation
- **Advanced Reasoning**: Chain-of-thought and tree-of-thoughts reasoning
- **Multi-Channel Notifications**: Slack, Discord, Email, Telegram integration

### ✅ **Phase 4: LLM Integration** (Completed)
- **Hardware Detection**: Comprehensive CPU/GPU/memory analysis
- **Model Management**: Intelligent model selection based on capabilities
- **Provider Architecture**: Unified interface for all LLM providers
- **CLI Interface**: Command-line interface with interactive mode

### ✅ **Phase 3: Workflows** (Completed)
- **✅ Project Management**: Full project lifecycle with database persistence
- **✅ Development Workflows**: Planning, building, testing, refactoring modes
- **✅ Session Management**: Multi-session support with context tracking
- **✅ Workflow Execution**: Automated workflow execution with dependencies

### ✅ **Phase 4: LLM Integration** (Completed)
- **✅ Hardware Detection**: Comprehensive CPU/GPU/memory analysis
- **✅ Model Management**: Intelligent model selection based on capabilities
- **✅ Provider Architecture**: Unified interface for all LLM providers
- **✅ CLI Interface**: Command-line interface with interactive mode

### ✅ **Phase 5: Advanced Features** (Completed)
- **✅ SSH Worker Pool**: Distributed worker network with auto-installation
- **✅ Advanced LLM Tooling**: Tool calling and reasoning API integration
- **✅ Multi-Client Support**: REST API, CLI, Terminal UI, WebSocket
- **✅ MCP Integration**: Full protocol support with multi-transport
- **✅ Cross-Platform**: Linux, macOS, Windows, Aurora OS, SymphonyOS
- **✅ Mobile Ready**: Framework for iOS and Android applications

## 🎉 **Project Status: FULLY COMPLETE**

**All 5 implementation phases have been successfully completed!** HelixCode is now a fully functional distributed AI development platform with enterprise-grade features including:

- **Complete Distributed Computing**: SSH-based worker networks with automatic management
- **Advanced AI Integration**: Multi-provider LLM support with tool calling and reasoning
- **Comprehensive Workflows**: Full development lifecycle automation (planning → building → testing → refactoring)
- **Multi-Platform Support**: Cross-platform compatibility with mobile frameworks ready
- **Enterprise Features**: Authentication, notifications, MCP protocol, and robust APIs

## 🏗️ Architecture

```
HelixCode Architecture
├── API Layer (REST + WebSocket + MCP)
├── Core Services
│   ├── Authentication & Session Management
│   ├── Worker Pool Management (SSH-based)
│   ├── Task Management & Checkpointing
│   ├── Project & Workflow Management
│   └── LLM Provider Integration
├── Database Layer (PostgreSQL + Redis)
├── Distributed Workers (Cross-platform)
└── Multi-Client Interfaces (CLI, TUI, REST, Mobile)
```

## 📦 Project Structure

```
helix_code/
├── Specification/          # Technical specifications and requirements
├── Implementation_Guide/   # Implementation plans and guides
├── Design/                 # Design assets and specifications
├── helix_code/              # Main Go implementation
│   ├── cmd/
│   │   ├── server/         # HTTP server application
│   │   └── cli/            # CLI client
│   ├── internal/
│   │   ├── auth/           # Authentication system
│   │   ├── config/         # Configuration management
│   │   ├── database/       # Database layer
│   │   ├── hardware/       # Hardware detection
│   │   ├── llm/            # LLM providers and reasoning
│   │   ├── logo/           # Logo processing & assets
│   │   ├── mcp/            # MCP protocol implementation
│   │   ├── notification/   # Multi-channel notifications
│   │   ├── project/        # Project management
│   │   ├── server/         # HTTP server & API
│   │   ├── session/        # Session management
│   │   ├── task/           # Task management & checkpoints
│   │   ├── worker/         # Worker pool management
│   │   └── workflow/       # Workflow execution
│   └── scripts/            # Build and utility scripts
├── Website/                # Marketing website
├── assets/                 # Project assets and logos
└── docs/                   # Documentation
```

## 🛠️ Quick Start

### Prerequisites
- Go 1.24.0+
- PostgreSQL 15+
- Redis 7+ (optional)

### Installation

1. **Clone the repository and setup environment**:
   ```bash
   git clone https://github.com/your-org/helixcode.git
   cd helixcode
   ./setup.sh
   ```

   This will:
   - Initialize all git submodules
   - Install system dependencies
   - Build the HelixCode application

2. **Manual setup (alternative)**:
   ```bash
   # Initialize submodules
   ./scripts/init-submodules.sh

   # Install dependencies (Ubuntu/Debian)
   ./install_missing_libs.sh

   # Build the application
   cd HelixCode
   make build
   ```

3. **Setup database**:
   ```bash
   createdb helixcode
   createuser helixcode
   ```

4. **Configure environment**:
   ```bash
   export HELIX_DATABASE_PASSWORD=your_password
   export HELIX_AUTH_JWT_SECRET=your_jwt_secret
   ```

5. **Run the server**:
   ```bash
   ./bin/helixcode
   ```

### CLI Usage

```bash
# Interactive mode
./cli

# List workers
./cli --list-workers

# Add a worker
./cli --worker worker-host --user helix --key ~/.ssh/id_rsa

# Generate with LLM
./cli --prompt "Hello world" --model llama-3-8b

# Health check
./cli --health
```

## 🔌 API Endpoints

### Authentication
- `POST /api/v1/auth/register` - User registration
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/refresh` - Token refresh

### Workers
- `GET /api/v1/workers` - List workers
- `POST /api/v1/workers` - Register worker
- `GET /api/v1/workers/:id` - Get worker details

### Tasks
- `GET /api/v1/tasks` - List tasks
- `POST /api/v1/tasks` - Create task
- `GET /api/v1/tasks/:id` - Get task details
- `POST /api/v1/tasks/:id/start` - Start task execution

### Projects
- `GET /api/v1/projects` - List projects
- `POST /api/v1/projects` - Create project
- `GET /api/v1/projects/:id` - Get project details

## 📊 Database Schema

### Core Tables
- **users**: User accounts and authentication
- **workers**: Distributed worker nodes with SSH config
- **tasks**: Task management with checkpoints and dependencies
- **projects**: Project lifecycle management
- **sessions**: Development sessions and context
- **llm_providers**: Configured LLM provider instances
- **notifications**: Multi-channel notification management

## 🔧 Development

### Build Commands
```bash
make build          # Build the application
make test           # Run all tests
make clean          # Clean build artifacts
make lint           # Lint code
make fmt            # Format code
```

### Testing
```bash
# Run all tests
go test ./...

# Run specific package tests
go test -v ./internal/auth

# Run with coverage
go test -cover ./...
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 📚 Documentation

- [Architecture Overview](helix_code/docs/ARCHITECTURE.md)
- [Development Guide](helix_code/docs/DEVELOPMENT.md)
- [User Guide](helix_code/docs/USER_GUIDE.md)
- [API Documentation](helix_code/docs/API.md)

## 🆘 Support

- **Issues**: [GitHub Issues](https://github.com/your-org/helixcode/issues)
- **Discussions**: [GitHub Discussions](https://github.com/your-org/helixcode/discussions)
- **Documentation**: See `/docs` directory

---

**Built with ❤️ using Go, PostgreSQL, and distributed computing principles**

*HelixCode - Empowering distributed AI development workflows*
