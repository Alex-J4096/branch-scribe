-- Model profiles are application-wide settings shared by every writing project.
ALTER TABLE model_profiles
    DROP COLUMN IF EXISTS project_id;
