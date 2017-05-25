package sesn

import (
	"mynewt.apache.org/newtmgr/nmxact/nmp"
	"mynewt.apache.org/newtmgr/nmxact/nmxutil"
)

func TxNmp(s Sesn, m *nmp.NmpMsg, o TxOptions) (nmp.NmpRsp, error) {
	retries := o.Tries - 1
	for i := 0; ; i++ {
		r, err := s.TxNmpOnce(m, o)
		if err == nil {
			return r, nil
		}

		if !nmxutil.IsRspTimeout(err) || i >= retries {
			return nil, err
		}
	}
}

func GetResource(s Sesn, uri string, o TxOptions) ([]byte, error) {
	retries := o.Tries - 1
	for i := 0; ; i++ {
		r, err := s.GetResourceOnce(uri, o)
		if err == nil {
			return r, nil
		}

		if !nmxutil.IsRspTimeout(err) || i >= retries {
			return nil, err
		}
	}
}
