newtmgr conn
-------------

Manage newtmgr connection profiles.

.. contents::
  :local:
  :depth: 2

Usage:
^^^^^^

.. code-block:: console

    newtmgr conn [command] [flags]

Global Flags:
^^^^^^^^^^^^^

.. code-block:: console

      -c, --conn string       connection profile to use
      -l, --loglevel string   log level to use (default "info")
          --name string       name of target BLE device; overrides profile setting
      -t, --timeout float     timeout in seconds (partial seconds allowed) (default 10)
      -r, --tries int         total number of tries in case of timeout (default 1)

Description
^^^^^^^^^^^

The conn command provides subcommands to add, delete, and view connection profiles. A connection profile specifies
information on how to connect and communicate with a remote device. Newtmgr commands use the information from a
connection profile to send newtmgr requests to remote devices.

Add Sub-Command
~~~~~~~~~~~~~~~

The ``newtmgr conn add <conn_profile> <var-name=value ...>`` command creates a connection profile named
``conn_profile``. The command requires the ``conn_profile`` name and a list of, space separated,
var-name=value pairs.

The var-names are: ``type``, and ``connstring``. The valid values for each var-name parameter are:

* ``type``:
  The connection type. Valid values are:

  - **serial**: Newtmgr protocol over a serial connection.
  - **oic_serial**: OIC protocol over a serial connection.
  - **udp**:newtmgr protocol over UDP.
  - **oic_udp**: OIC protocol over UDP.
  - **ble** newtmgr protocol over BLE. This type uses native OS BLE support
  - **oic_ble**: OIC protocol over BLE. This type uses native OS BLE support.
  - **bhd**: newtmgr protocol over BLE. This type uses the blehostd implemenation.
  - **oic_bhd**: OIC protocol over BLE. This type uses the blehostd implementation.

  **Note:** newtmgr does not support BLE on Windows.

* ``connstring``:
  The physical or virtual address for the connection. The format of the ``connstring`` value depends
  on the connection ``type`` value as follows:

  - **serial** and **oic_serial**: A quoted string with two, comma separated, ``attribute=value`` pairs.
    The attribute names and value format for each attribute are:

    * ``dev``: (Required) The name of the serial port to use. For example: **/dev/ttyUSB0** on a Linux platform or
      **COM1** on a Windows platform .
    * ``baud``: (Optional) A number that specifies the buad rate for the connection. Defaults to **115200** if the
      attribute is not specified.

    Example: ``connstring="dev=/dev/ttyUSB0, baud=9600"``
    **Note:** The 1.0 format, which only requires a serial port name, is still supported. For example, ``connstring=/dev/ttyUSB0``.

  - **udp** and **oic_udp**: The peer ip address and port number that the newtmgr or oicmgr on the remote device is
    listening on. It must be of the form: **[<ip-address>]:<port-number>**.

  - **ble** and **oic_ble**: The format is a quoted string of, comma separated, ``attribute=value`` pairs. The attribute
    names and the value for each attribute are:

    * ``peer_name``: A string that specifies the name the peer BLE device advertises. **Note**: If this attribute is
      specified, you do not need to specify a value for the ``peer_id`` attribute.

    * ``peer_id``: The peer BLE device address or UUID. The format depends on the OS that the newtmgr tool is running on:

        **Linux**: 6 byte BLE address. Each byte must be a hexidecimal number and separated by a colon.

        **MacOS**: 128 bit UUID.

      **Note**: This value is only used when a peer name is not specified for the connection profile or with the
      ``--name`` flag option.

    * ``ctlr_name``: (Optional) Controller name. This value depends on the OS that the newtmgr tool is running on.


    **Notes**:

    * You must specify ``connstring=" "`` if you do not specify any attribute values.

    * You can use the ``--name`` flag to specify a device name when you issue a newtmgr command that communicates with
      a BLE device. You can use this flag to override or in lieu of specifying a ``peer_name`` or ``peer_id`` attribute
      in the connection profile.

  - **bhd** and **oic_bhd**: The format is a quoted string of, comma separated, ``attribute=value`` pairs. The
    attribute names and the value format for each attribute are:

    * ``peer_name``: A string that specifies the name the peer BLE device advertises.
      **Note**: If this attribute is specified, you do not need to specify values for the ``peer_addr`` and
      ``peer_addr_type`` attributes.

    * ``peer_addr``: A 6 byte peer BLE device address. Each byte must be a hexidecimal number and separated by a colon.
      You must also specify a ``peer_addr_type`` value for the device address.
      **Note:** This value is only used when a peer name is not specified for the connection profile or with the
      ``--name`` flag option.

    * ``peer_addr_type``: The peer address type. Valid values are:

      - **public**: Public address assigned by the manufacturer.
      - **random**: Static random address.
      - **rpa_pub**: Resolvable Private Address with public identity address.
      - **rpa_rnd**: Resolvable Private Address with static random identity address.

      **Note:** This value is only used when a peer name is not specified for the connection profile or with the
      ``--name`` flag option.

    * ``own_addr_type``: (Optional) The address type of the BLE controller for the host that the newtmgr tool is
      running on. See the ``peer_addr_type`` attribute for valid values. Defaults to **random**.

    * ``ctlr_path``: The path of the port that is used to connect the BLE controller to the host that the newtmgr tool is
      running on.

  **Note**: You can use the ``--name`` flag to specify a device name when you issue a newtmgr command that communicates
  with a BLE device. You can use this flag to override or in lieu of specifying a ``peer_name`` or ``peer_addr``
  attribute in the connection profile.

Delete Sub-Command
~~~~~~~~~~~~~~~~~~

The ``newtmgr conn delete <conn_profile>`` command deletes the ``conn_profile`` connection profile.

Show Sub-Command
~~~~~~~~~~~~~~~~

The ``newtmgr conn show [conn_profile]`` command shows the information for the ``conn_profile`` connection profile.
It shows information for all the connection profiles if ``conn_profile`` is not specified.

Examples
^^^^^^^^

+---------------+-------------------------------------------------------------------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| Sub-command   | Usage                                                                                                                   | Explanation                                                                                                                                                                                                                                                                           |
+===============+=========================================================================================================================+=======================================================================================================================================================================================================================================================================================+
| add           | ``newtmgr conn add myserial02 type=oic_serial connstring=/dev/ttys002``                                                 | Creates a connection profile, named ``myserial02``, to communicate over a serial connection at 115200 baud rate with the oicmgr on a device that is connected to the host on port /dev/ttys002.                                                                                       |
+---------------+-------------------------------------------------------------------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| add           | ``newtmgr conn add myserial03 type=serial connstring="dev=/dev/ttys003, baud=57600"``                                   | Creates a connection profile, named ``myserial03``, to communicate over a serial connection at 57600 baud rate with the newtmgr on a device that is connected to the host on port /dev/ttys003.                                                                                       |
+---------------+-------------------------------------------------------------------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| add           | ``newtmgr conn add myudp5683 type=oic_udpconnstring=[127.0.0.1]:5683``                                                  | Creates a connection profile, named ``myudp5683``, to communicate over UDP with the oicmgr on a device listening on localhost and port 5683.                                                                                                                                          |
+---------------+-------------------------------------------------------------------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| add           | ``newtmgr conn add mybleprph type=ble connstring="peer_name=nimble-bleprph"``                                           | Creates a connection profile, named ``mybleprph``, to communicate over BLE, using the native OS BLE support, with the newtmgr on a device named ``nimble-bleprph``.                                                                                                                   |
+---------------+-------------------------------------------------------------------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| add           | ``newtmgr conn add mybletype=ble connstring=" "``                                                                       | Creates a connection profile, named ``myble``, to communicate over BLE, using the native OS BLE support, with the newtmgr on a device. You must use the ``--name`` flag to specify the device name when you issue a newtmgr command that communicates with the device.                |
+---------------+-------------------------------------------------------------------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| add           | ``newtmgr conn add myblehostd type=oic_bhd connstring="peer_name=nimble-bleprph,ctlr_path=/dev/cu.usbmodem14221"``      | Creates a connection profile, named ``myblehostd``, to communicate over BLE, using the blehostd implementation, with the oicmgr on a device named ``nimble-bleprph``. The BLE controller is connected to the host on USB port /dev/cu.usbmodem14211 and uses static random address.   |
+---------------+-------------------------------------------------------------------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| delete        | ``newtmgr conn delete myserial02``                                                                                      | Deletes the connection profile named ``myserial02``                                                                                                                                                                                                                                   |
+---------------+-------------------------------------------------------------------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| delete        | ``newtmgr conn delete myserial02``                                                                                      | Deletes the connection profile named ``myserial02``                                                                                                                                                                                                                                   |
+---------------+-------------------------------------------------------------------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| show          | ``newtmgr conn show myserial01``                                                                                        | Displays the information for the ``myserial01`` connection profile.                                                                                                                                                                                                                   |
+---------------+-------------------------------------------------------------------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| show          | ``newtmgr conn show``                                                                                                   | Displays the information for all connection profiles.                                                                                                                                                                                                                                 |
+---------------+-------------------------------------------------------------------------------------------------------------------------+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
