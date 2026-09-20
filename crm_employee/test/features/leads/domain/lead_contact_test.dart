import 'package:crm_employee/features/leads/domain/lead_contact.dart';
import 'package:flutter_test/flutter_test.dart';

// Issue #143. "Punya kontak" is email OR phone that is not blank. The
// blank-handling is not pedantry: crm_be trims an email and stores what is
// left, so a submission of "   " can come back as "" rather than null — a lead
// that a naive `lead.email != null` would call contactable.
void main() {
  group('leadHasContact', () {
    test('is true when either one is present, or both', () {
      expect(leadHasContact(email: 'budi@example.com', phone: null), isTrue);
      expect(leadHasContact(email: null, phone: '0812 3456 7890'), isTrue);
      expect(leadHasContact(email: 'budi@example.com', phone: '0812'), isTrue);
    });

    test('is false when neither exists — null or missing', () {
      expect(leadHasContact(email: null, phone: null), isFalse);
      expect(leadHasContact(), isFalse);
    });

    test('treats an empty or whitespace-only value as absent', () {
      expect(leadHasContact(email: '', phone: ''), isFalse);
      expect(leadHasContact(email: '   ', phone: '\t\n'), isFalse);
    });

    test('lets one real value carry a blank neighbour', () {
      expect(leadHasContact(email: '   ', phone: '0812'), isTrue);
      expect(leadHasContact(email: 'budi@example.com', phone: '  '), isTrue);
    });

    test(
      'counts a phone that will not normalise for WhatsApp — it can still be dialled',
      () {
        expect(leadHasContact(email: null, phone: 'ext 12'), isTrue);
      },
    );
  });

  // The call button and the sentence explaining why it is off must agree. They
  // used to disagree at the edges: `phone != null && phone.isNotEmpty` enabled
  // Telepon for "   " while nothing said there was no number.
  group('hasPhoneNumber', () {
    test('agrees with leadHasContact about what a blank phone is', () {
      expect(hasPhoneNumber('0812 3456 7890'), isTrue);
      expect(hasPhoneNumber(null), isFalse);
      expect(hasPhoneNumber(''), isFalse);
      expect(hasPhoneNumber('   '), isFalse);
    });
  });
}
