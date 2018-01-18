newtmgr reset
--------------

Send a reset request to a device.

Usage:
^^^^^^

.. code-block:: console

        newtmgr reset -c <conn_profile> [flags]

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

Resets a device. Newtmgr uses the ``conn_profile`` connection profile to connect to the device.

Examples
^^^^^^^^

+---------------------------------+------------------------------------------------------------------------------------------------------------------------+
| Usage                           | Explanation                                                                                                            |
+=================================+========================================================================================================================+
| ``newtmgr reset-c profile01``   | Resets a device. Newtmgr connects to the device over a connection specified in the ``profile01`` connection profile.   |
+---------------------------------+------------------------------------------------------------------------------------------------------------------------+
