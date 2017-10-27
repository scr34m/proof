DROP TABLE `event`;
DROP TABLE `group`;
DROP TABLE `data`;

CREATE TABLE `event` (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  data_id CHAR(32) NOT NULL,
  group_id INT NOT NULL,
  message TEXT NOT NULL,
  checksum CHAR(32) NOT NULL
);

CREATE TABLE `group` (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  logger CHAR(64) NOT NULL,
  level CHAR(32) NOT NULL,
  message TEXT NOT NULL,
  checksum CHAR(32) NOT NULL,
  status INT NOT NULL,
  seen INT NOT NULL,
  last_seen TEXT NOT NULL,
  first_seen TEXT NOT NULL,
  project_id INT NOT NULL,
  server_name CHAR(128) NOT NULL,
  platform CHAR(64) NOT NULL,
  url CHAR(200) NOT NULL,
  site CHAR(128) NOT NULL
);

CREATE TABLE `data` (
  id CHAR(32) NOT NULL,
  data TEXT NOT NULL,
  timestamp TEXT NOT NULL
);

