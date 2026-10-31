newtmgr params
--------------

Read mcumgr buffer parameters from a device.

Usage:
^^^^^^

.. code-block:: console

        newtmgr params -c <conn_profile> [flags]

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

Reads and displays the mcumgr buffer parameters of a device. Newtmgr uses the ``conn_profile`` connection profile to
connect to the device. It displays the following parameters:

-  **buf_size**: Size (number of bytes, header included) of the largest request the device accepts
-  **buf_count**: Number of requests that may be outstanding at the same time

Over BLE, ``newtmgr image upload`` uses these parameters to send requests larger than the ATT MTU; such requests are
split over several BLE writes and reassembled by the device.

Examples
^^^^^^^^

+-----------------------------------+--------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| Usage                             | Explanation                                                                                                                                                        |
+===================================+====================================================================================================================================================================+
| ``newtmgr params -c profile01``   | Reads and displays the mcumgr buffer parameters from a device. Newtmgr connects to the device over a connection specified in the ``profile01`` connection profile. |
+-----------------------------------+--------------------------------------------------------------------------------------------------------------------------------------------------------------------+

Example output:

.. code-block:: console

    $ newtmgr params -c myble
    buf_size: 1024
    buf_count: 1
