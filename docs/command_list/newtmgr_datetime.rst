newtmgr datetime
-----------------

Manage datetime on a device.

Usage:
^^^^^^

.. code-block:: console

        newtmgr datetime [rfc-3339-date-string] -c <conn_profile> [flags]

Global Flags:
^^^^^^^^^^^^^

.. code-block:: console

      -c, --conn string       connection profile to use
      -h, --help              help for newtmgr
      -l, --loglevel string   log level to use (default "info")
          --name string       name of target BLE device; overrides profile setting
      -t, --timeout float     timeout in seconds (partial seconds allowed) (default 10)
      -r, --tries int         total number of tries in case of timeout (default 1)

Description
^^^^^^^^^^^

Reads or sets the datetime on a device. Specify a ``datetime-value`` in the command to set the datetime on the device.
Newtmgr uses the ``conn_profile`` connection profile to connect to the device.

**Note**: You must specify the ``datetime-value`` in the RFC 3339 format.

Examples
^^^^^^^^

+------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| Usage                                                      | Explanation                                                                                                                                                         |
+============================================================+=====================================================================================================================================================================+
| ``newtmgr datetime-c profile01``                           | Reads the datetime value from a device. Newtmgr connects to the device over a connection specified in the ``profile01`` connection profile.                         |
+------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ``newtmgr datetime 2017-03-01T22:44:00-c profile01``       | Sets the datetime on a device to March 1st 2017 22:44:00 UTC. Newtmgr connects to the device over a connection specified in the ``profile01`` connection profile.   |
+------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ``newtmgr datetime 2017-03-01T22:44:00-08:00-c profile01`` | Sets the datetime on a device to March 1st 2017 22:44:00 PST. Newtmgr connects to the device over a connection specified in the ``profile01`` connection profile.   |
+------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------+
