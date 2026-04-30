# DOD Mobile

Flutter mobile application for DOD platform - iOS & Android

## 🏗 Architecture

This project follows **Clean Architecture** principles with clear separation of concerns:

```
lib/
├── core/                    # Core utilities, DI, network, theme
│   ├── config/             # App configuration
│   ├── di/                 # Dependency Injection (GetIt)
│   ├── error/              # Failures and Exceptions
│   ├── network/            # API Client, WebSocket
│   ├── theme/              # App theme
│   ├── utils/              # Formatters, validators
│   └── router/             # GoRouter navigation
│
├── features/               # Feature modules
│   ├── auth/
│   │   ├── domain/         # Entities, Repositories, UseCases
│   │   ├── data/           # Models, DataSources, RepositoryImpl
│   │   └── presentation/   # BLoC, Pages, Widgets
│   ├── sportsbook/
│   ├── casino/
│   ├── wallet/
│   ├── profile/
│   ├── bonuses/
│   └── notifications/
│
└── shared/                 # Shared widgets and extensions
```

## 🛠 Tech Stack

- **Framework:** Flutter 3.x (Dart 3.2+)
- **State Management:** BLoC + flutter_bloc
- **Dependency Injection:** GetIt + injectable
- **Navigation:** GoRouter
- **Networking:** Dio (HTTP) + web_socket_channel
- **Local Storage:** Hive + flutter_secure_storage
- **Code Generation:** freezed, json_serializable, injectable_generator
- **Functional Programming:** dartz (Either type)

## 🚀 Quick Start

```bash
# Install dependencies
flutter pub get

# Generate code (run after adding new @injectable, @freezed, etc.)
flutter pub run build_runner build --delete-conflicting-outputs

# Run on device/emulator
flutter run

# Run with flavor
flutter run --flavor dev -t lib/main_dev.dart
```

## 📁 Project Structure

### Core Layer

**Dependency Injection:**
```dart
// lib/core/di/injection.dart
final getIt = GetIt.instance;

@InjectableInit()
Future<void> configureDependencies() async => getIt.init();
```

**API Client:**
```dart
// lib/core/network/api_client.dart
@singleton
class ApiClient {
  late final Dio _dio;
  
  Future<Response> get(String path, {Map<String, dynamic>? params});
  Future<Response> post(String path, {dynamic data});
  // ...
}
```

**Error Handling:**
```dart
// lib/core/error/failures.dart
abstract class Failure extends Equatable {
  final String message;
  final String? code;
}

class ServerFailure extends Failure { ... }
class NetworkFailure extends Failure { ... }
class AuthFailure extends Failure { ... }
```

### Feature Module Structure

Each feature follows Clean Architecture:

**Domain Layer** (pure Dart, no Flutter dependencies):
```dart
// Entity
class User extends Equatable {
  final int id;
  final String email;
  // ...
}

// Repository Interface
abstract class AuthRepository {
  Future<Either<Failure, User>> login({...});
}

// UseCase
class Login {
  final AuthRepository _repository;
  Future<Either<Failure, User>> call(LoginParams params) { ... }
}
```

**Data Layer:**
```dart
// Model (with JSON serialization)
@JsonSerializable()
class UserModel extends User { ... }

// DataSource
abstract class AuthRemoteDataSource {
  Future<AuthTokens> login({...});
}

// Repository Implementation
@LazySingleton(as: AuthRepository)
class AuthRepositoryImpl implements AuthRepository { ... }
```

**Presentation Layer:**
```dart
// BLoC
@injectable
class AuthBloc extends Bloc<AuthEvent, AuthState> { ... }

// Page
class LoginPage extends StatelessWidget { ... }

// Widget
class LoginForm extends StatefulWidget { ... }
```

## 🧪 Testing

```bash
# Run unit tests
flutter test

# Run with coverage
flutter test --coverage

# Run BLoC tests specifically
flutter test features/auth/presentation/bloc/
```

### Example BLoC Test

```dart
blocTest<AuthBloc, AuthState>(
  'emits [Loading, Authenticated] when Login succeeds',
  build: () {
    when(() => mockLogin(any)).thenAnswer((_) async => const Right(testUser));
    return getIt<AuthBloc>();
  },
  act: (bloc) => bloc.add(const AuthEvent.login('test@test.com', 'password')),
  expect: () => [
    const AuthState.loading(),
    isA<AuthAuthenticated>().having((s) => s.user.email, 'email', 'test@test.com'),
  ],
);
```

## 🔐 Security

- **Tokens:** Stored in flutter_secure_storage (encrypted)
- **Biometric Auth:** local_auth package ready
- **Certificate Pinning:** Configurable in Dio interceptor
- **Obfuscation:** Enabled in production builds

## 📦 Build

### Android

```bash
# APK
flutter build apk --release

# App Bundle (Play Store)
flutter build appbundle --release
```

### iOS

```bash
# IPA
flutter build ios --release

# Open in Xcode for archiving
open ios/Runner.xcworkspace
```

## 🔧 Environment Variables

Create `lib/core/config/env.dart`:

```dart
class Env {
  static const String apiUrl = String.fromEnvironment('API_URL', defaultValue: 'http://localhost:8080');
  static const String wsUrl = String.fromEnvironment('WS_URL', defaultValue: 'ws://localhost:8080');
  static const String environment = String.fromEnvironment('ENVIRONMENT', defaultValue: 'development');
}
```

## 📄 License

Proprietary - DOD
