CREATE TABLE `event` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `data_id` varchar(32) NOT NULL,
  `group_id` int(11) DEFAULT NULL,
  `message` longtext NOT NULL,
  `checksum` varchar(32) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_1` (`group_id`) USING BTREE,
  KEY `idx_2` (`id`),
  KEY `idx_3` (`data_id`(16))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE `group` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `logger` varchar(64) NOT NULL,
  `level` varchar(32) NOT NULL,
  `message` longtext NOT NULL,
  `checksum` varchar(32) NOT NULL,
  `status` int(10) unsigned NOT NULL,
  `seen` int(10) unsigned NOT NULL,
  `last_seen` datetime NOT NULL,
  `first_seen` datetime NOT NULL,
  `project_id` int(11) DEFAULT NULL,
  `server_name` varchar(128) DEFAULT NULL,
  `platform` varchar(64) DEFAULT NULL,
  `url` varchar(200) DEFAULT NULL,
  `site` varchar(128) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_1` (`checksum`,`project_id`),
  KEY `idx_2` (`status`,`last_seen`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE `data` (
  `id` varchar(32) NOT NULL,
  `data` longtext NOT NULL,
  `timestamp` datetime NOT NULL,
  `protocol` tinyint NOT NULL DEFAULT 4,
  PRIMARY KEY (`id`),
  KEY `id` (`id`(16))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

