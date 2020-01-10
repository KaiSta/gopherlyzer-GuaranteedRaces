package test;

public class Test2 extends Thread {
  static int x;
  static int y;
  static final Object m = new Object();

  static void sleepSec(float sec) {
    try {
	Thread.sleep((long)(sec*1000));
    } catch (Exception e) {
       throw new RuntimeException(e);
    }
  }


  @Override
  public void run() {
   	//T1  
	x = 1;
	System.out.println("w(x)");
	synchronized(m) {		
		System.out.println("l(m)");
		int tmp = y;
		System.out.println("r(y)");
		System.out.println("u(m)");
	}
  } 

   public static class T2 extends Thread implements Runnable {
		public void run() {
			sleepSec(5);
			synchronized(m) {
				System.out.println("l(m)");
				int tmp = y;
				System.out.println("r(y)");
				System.out.println("u(m)");
			}
			int tmp = x;
			System.out.println("r(x)");
		}
   }


   public static void main(String args[]) throws Exception {
	final Test2 thread1 = new Test2();
    final T2 thread2 = new T2();

	thread1.start();
	thread2.start();

	thread1.join();
	thread2.join();

   }
}
