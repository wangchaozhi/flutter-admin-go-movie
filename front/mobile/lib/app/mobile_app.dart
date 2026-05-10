import 'package:flutter/material.dart';
import 'package:forui/forui.dart';

import '../features/auth/mobile_login_page.dart';
import '../features/home/mobile_home_page.dart';

class MobileApp extends StatelessWidget {
  const MobileApp({super.key});

  @override
  Widget build(BuildContext context) {
    final foruiTheme = FThemes.zinc.light.touch;

    return MaterialApp(
      title: 'Go Movie',
      debugShowCheckedModeBanner: false,
      localizationsDelegates: FLocalizations.localizationsDelegates,
      supportedLocales: FLocalizations.supportedLocales,
      theme: foruiTheme.toApproximateMaterialTheme().copyWith(
        colorScheme: ColorScheme.fromSeed(
          seedColor: const Color(0xFF25D0AB),
          brightness: Brightness.light,
        ),
        scaffoldBackgroundColor: const Color(0xFFF6F7FB),
      ),
      builder: (context, child) => FTheme(data: foruiTheme, child: child!),
      initialRoute: '/home',
      routes: {
        '/login': (_) => const MobileLoginPage(),
        '/home': (_) => const MobileHomePage(),
      },
    );
  }
}
