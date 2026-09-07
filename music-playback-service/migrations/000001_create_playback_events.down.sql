DROP TRIGGER IF EXISTS playback_events_append_only ON playback_events;
DROP FUNCTION IF EXISTS reject_playback_event_mutation();
DROP TABLE IF EXISTS playback_events;