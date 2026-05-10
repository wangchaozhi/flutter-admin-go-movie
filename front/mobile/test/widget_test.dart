import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:mobile/app/mobile_app.dart';

void main() {
  testWidgets('app builds', (tester) async {
    SharedPreferences.setMockInitialValues({});
    await tester.pumpWidget(const MobileApp());
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));

    expect(find.text('Go Movie'), findsOneWidget);
    expect(find.text('霓虹城市追踪'), findsOneWidget);
  });
}
