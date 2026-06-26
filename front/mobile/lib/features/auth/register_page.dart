import 'package:flutter/material.dart';

import '../../core/api_client.dart';
import '../../core/l10n/app_strings.dart';
import '../../core/session.dart';

/// Self-service sign-up. On success the new account is signed straight in (the
/// backend returns a token) and the user lands on the home page.
class RegisterPage extends StatefulWidget {
  const RegisterPage({super.key});

  @override
  State<RegisterPage> createState() => _RegisterPageState();
}

class _RegisterPageState extends State<RegisterPage> {
  final _username = TextEditingController();
  final _password = TextEditingController();
  final _confirm = TextEditingController();
  final _nickname = TextEditingController();
  final _email = TextEditingController();
  final _invite = TextEditingController();

  bool _loading = false;
  String? _error;

  @override
  void dispose() {
    _username.dispose();
    _password.dispose();
    _confirm.dispose();
    _nickname.dispose();
    _email.dispose();
    _invite.dispose();
    super.dispose();
  }

  String? _validate(AppStrings s) {
    final username = _username.text.trim();
    if (username.length < 3 || username.length > 32) {
      return s.t('register.errUsernameLen');
    }
    if (_password.text.length < 6) return s.t('register.errPasswordLen');
    if (_password.text != _confirm.text) return s.t('register.errMismatch');
    final email = _email.text.trim();
    if (email.isNotEmpty && !email.contains('@')) {
      return s.t('register.errEmail');
    }
    if (_invite.text.trim().isEmpty) return s.t('register.errInvite');
    return null;
  }

  Future<void> _register() async {
    final s = AppStrings.of(context);
    final problem = _validate(s);
    if (problem != null) {
      setState(() => _error = problem);
      return;
    }
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final username = _username.text.trim();
      final resp = await ApiClient().post('/api/mobile/register', {
        'username': username,
        'password': _password.text,
        'nickname': _nickname.text.trim(),
        'email': _email.text.trim(),
        'invite_code': _invite.text.trim(),
      });
      if (!mounted) return;
      if (resp['code'] != 0) {
        setState(
          () => _error = resp['msg']?.toString() ?? s.t('register.failed'),
        );
        return;
      }
      final data = resp['data'] as Map<String, dynamic>?;
      final token = data?['token'] as String? ?? '';
      await Session.save(token, username);
      if (!mounted) return;
      Navigator.pushNamedAndRemoveUntil(context, '/home', (route) => false);
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = '${s.t('register.failed')}: $e');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final s = AppStrings.of(context);
    return Scaffold(
      backgroundColor: const Color(0xFF0D0F14),
      appBar: AppBar(
        backgroundColor: const Color(0xFF0D0F14),
        foregroundColor: Colors.white,
        elevation: 0,
        title: Text(s.t('register.title')),
      ),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.fromLTRB(20, 12, 20, 32),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 430),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  const _RegisterHeader(),
                  const SizedBox(height: 18),
                  Material(
                    color: const Color(0xFF171B24),
                    elevation: 10,
                    shadowColor: const Color(0x66000000),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(8),
                      side: const BorderSide(color: Color(0xFF2B3140)),
                    ),
                    child: Padding(
                      padding: const EdgeInsets.all(18),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          _field(
                            _username,
                            s.t('register.username'),
                            s.t('register.usernameHint'),
                            Icons.person_outline_rounded,
                          ),
                          const SizedBox(height: 14),
                          _field(
                            _password,
                            s.t('register.password'),
                            s.t('register.passwordHint'),
                            Icons.lock_outline_rounded,
                            obscure: true,
                          ),
                          const SizedBox(height: 14),
                          _field(
                            _confirm,
                            s.t('register.confirm'),
                            s.t('register.confirmHint'),
                            Icons.lock_outline_rounded,
                            obscure: true,
                          ),
                          const SizedBox(height: 14),
                          _field(
                            _invite,
                            s.t('register.invite'),
                            s.t('register.inviteHint'),
                            Icons.confirmation_number_outlined,
                          ),
                          const SizedBox(height: 14),
                          _field(
                            _nickname,
                            s.t('register.nickname'),
                            s.t('register.nicknameHint'),
                            Icons.badge_outlined,
                          ),
                          const SizedBox(height: 14),
                          _field(
                            _email,
                            s.t('register.email'),
                            s.t('register.emailHint'),
                            Icons.mail_outline_rounded,
                          ),
                          if (_error != null) ...[
                            const SizedBox(height: 14),
                            _InlineError(message: _error!),
                          ],
                          const SizedBox(height: 22),
                          FilledButton.icon(
                            onPressed: _loading ? null : _register,
                            icon: _loading
                                ? const SizedBox(
                                    width: 18,
                                    height: 18,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                      color: Color(0xFF101318),
                                    ),
                                  )
                                : const Icon(Icons.person_add_alt_1_rounded),
                            label: Text(s.t('register.submit')),
                            style: FilledButton.styleFrom(
                              backgroundColor: const Color(0xFF25D0AB),
                              foregroundColor: const Color(0xFF07110F),
                              minimumSize: const Size.fromHeight(52),
                              textStyle: const TextStyle(
                                fontWeight: FontWeight.w900,
                              ),
                              shape: RoundedRectangleBorder(
                                borderRadius: BorderRadius.circular(8),
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _field(
    TextEditingController controller,
    String label,
    String hint,
    IconData icon, {
    bool obscure = false,
  }) {
    return TextField(
      controller: controller,
      obscureText: obscure,
      enableSuggestions: !obscure,
      autocorrect: !obscure,
      cursorColor: const Color(0xFF25D0AB),
      style: const TextStyle(color: Colors.white),
      decoration: InputDecoration(
        labelText: label,
        hintText: hint,
        prefixIcon: Icon(icon),
        prefixIconColor: const Color(0xFF25D0AB),
        filled: true,
        fillColor: const Color(0xFF101318),
        labelStyle: const TextStyle(color: Color(0xFF9CA3AF)),
        hintStyle: const TextStyle(color: Color(0xFF6B7280)),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8),
          borderSide: const BorderSide(color: Color(0xFF2B3140)),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8),
          borderSide: const BorderSide(color: Color(0xFF2B3140)),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(8),
          borderSide: const BorderSide(color: Color(0xFF25D0AB), width: 1.4),
        ),
      ),
    );
  }
}

class _RegisterHeader extends StatelessWidget {
  const _RegisterHeader();

  @override
  Widget build(BuildContext context) {
    return const Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          '欢迎加入 Go Movie',
          style: TextStyle(
            color: Colors.white,
            fontSize: 24,
            fontWeight: FontWeight.w900,
          ),
        ),
        SizedBox(height: 8),
        Text(
          '完成邀请码验证后会自动登录，观影进度、收藏和会员权益都会同步到账号。',
          style: TextStyle(color: Color(0xFF9CA3AF), height: 1.5),
        ),
      ],
    );
  }
}

class _InlineError extends StatelessWidget {
  const _InlineError({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: const Color(0x22F87171),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0x66F87171)),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        child: Text(
          message,
          style: const TextStyle(color: Color(0xFFFCA5A5), fontSize: 13),
        ),
      ),
    );
  }
}
