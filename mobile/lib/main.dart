import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'core/app_state.dart';
import 'screens/home_screen.dart';
import 'screens/login_screen.dart';
import 'screens/setup_screen.dart';

void main() {
  runApp(
    ChangeNotifierProvider(
      create: (_) => AppController()..init(),
      child: const ItamFieldApp(),
    ),
  );
}

class ItamFieldApp extends StatelessWidget {
  const ItamFieldApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'ITAM Field',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        useMaterial3: true,
        colorScheme: ColorScheme.fromSeed(
          seedColor: const Color(0xFF3b82f6),
          brightness: Brightness.dark,
        ),
        scaffoldBackgroundColor: const Color(0xFF0f1117),
        inputDecorationTheme: const InputDecorationTheme(border: OutlineInputBorder()),
      ),
      home: const _Gate(),
    );
  }
}

class _Gate extends StatelessWidget {
  const _Gate();

  @override
  Widget build(BuildContext context) {
    final status = context.watch<AppController>().status;
    switch (status) {
      case AppStatus.loading:
        return const Scaffold(body: Center(child: CircularProgressIndicator()));
      case AppStatus.needsSetup:
        return const SetupScreen();
      case AppStatus.needsLogin:
        return const LoginScreen();
      case AppStatus.ready:
        return const HomeScreen();
    }
  }
}
