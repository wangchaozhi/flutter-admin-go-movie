-- Convert the seed plaintext passwords ("123456") to bcrypt hashes.
-- Only rows that still hold the default plaintext value are touched, so any
-- password already changed through the admin UI (and stored hashed) is left
-- untouched. Application login also accepts legacy plaintext as a fallback, so
-- this migration is safe to run repeatedly.
UPDATE admin_users
SET password = '$2a$10$sTQc3h8gzXkd7np2RVusVO1xPvO5uacPxqIzrJeWL8MfMDlEjp1z2'
WHERE password = '123456';

UPDATE mobile_users
SET password = '$2a$10$Av7CR8jOsLakanzK77ryD./IfTRXmjOdZSIJCRErThhtXCX4Phi1e'
WHERE password = '123456';
