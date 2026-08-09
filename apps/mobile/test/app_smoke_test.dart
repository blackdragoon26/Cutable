import 'package:cutable_mobile/app.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('CutableApp boots to the splash screen without crashing', (tester) async {
    await tester.pumpWidget(const ProviderScope(child: CutableApp()));
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsOneWidget);
  });
}
