-- Ensure the documented default mobile demo account exists.
-- The password is "123456" stored as bcrypt; existing "user" rows are left
-- untouched so this migration does not reset a changed password.
INSERT INTO mobile_users (username, password, nickname)
VALUES (
  'user',
  '$2a$10$Av7CR8jOsLakanzK77ryD./IfTRXmjOdZSIJCRErThhtXCX4Phi1e',
  'mobile user'
)
ON CONFLICT (username) DO NOTHING;
