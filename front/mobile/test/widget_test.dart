import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:mobile/app/mobile_app.dart';
import 'package:mobile/features/auth/mobile_login_page.dart';

void main() {
  testWidgets('app boots to the login gate when no session is stored', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});

    await tester.pumpWidget(const MobileApp());
    // Let the auth gate resolve the (empty) stored session.
    await tester.pumpAndSettle();

    // The app should build without throwing and land on the login screen.
    expect(tester.takeException(), isNull);
    expect(find.byType(MaterialApp), findsOneWidget);
    expect(find.byType(MobileLoginPage), findsOneWidget);
  });
}
